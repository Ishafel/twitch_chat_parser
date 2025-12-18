package storage

import (
	"context"
	"encoding/json"
	"log"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"twitch-chat-logger/model"
)

// BatchConfig задаёт параметры батчинга для вставки сообщений.
type BatchConfig struct {
	MaxBatch      int
	FlushEvery    time.Duration
	ChanBuffer    int
	StatsLogEvery time.Duration
	FlushTimeout  time.Duration
}

// Batcher асинхронно вставляет сообщения чата через pgx.Batch.
type Batcher struct {
	input   chan model.ChatMessage
	config  BatchConfig
	sender  batchSender
	dropped atomic.Uint64
}

type batchSender interface {
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// NewBatcher создаёт батчер и запускает фоновые флаши.
func NewBatcher(ctx context.Context, pool *pgxpool.Pool, cfg BatchConfig) *Batcher {
	return newBatcher(ctx, pool, cfg)
}

// Enqueue пытается добавить сообщение в очередь; при переполнении возвращает false.
func (b *Batcher) Enqueue(msg model.ChatMessage) bool {
	select {
	case b.input <- msg:
		return true
	default:
		dropped := b.dropped.Add(1)
		if dropped%100 == 0 {
			log.Printf("батчер: очередь заполнена, всего отброшено %d сообщений", dropped)
		}
		return false
	}
}

// Dropped возвращает число сообщений, отброшенных из-за переполнения.
func (b *Batcher) Dropped() uint64 {
	return b.dropped.Load()
}

func (b *Batcher) run(ctx context.Context) {
	flushTicker := time.NewTicker(b.config.FlushEvery)
	statsTicker := time.NewTicker(b.config.StatsLogEvery)
	defer flushTicker.Stop()
	defer statsTicker.Stop()

	var (
		batch            = &pgx.Batch{}
		pending          = 0
		totalInserted    uint64
		intervalInserted uint64
	)

	const q = `
insert into chat_messages (
  message_id, channel, user_id, username, display_name, text, badges, color,
  is_mod, is_subscriber, bits, sent_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
on conflict (message_id) do nothing;`

	flush := func() {
		if pending == 0 {
			return
		}

		dbCtx, cancel := context.WithTimeout(context.Background(), b.config.FlushTimeout)
		defer cancel()

		br := b.sender.SendBatch(dbCtx, batch)
		if err := br.Close(); err != nil {
			log.Printf("ошибка флаша батчера: %v", err)
		}

		totalInserted += uint64(pending)
		intervalInserted += uint64(pending)

		batch = &pgx.Batch{}
		pending = 0
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			log.Printf("батчер: контекст отменён, всего вставлено строк = %d", totalInserted)
			return
		case <-flushTicker.C:
			flush()
		case <-statsTicker.C:
			log.Printf(
				"батчер: вставлено %d строк за %s (всего %d)",
				intervalInserted, b.config.StatsLogEvery, totalInserted,
			)
			intervalInserted = 0
		case msg := <-b.input:
			badgesJSON, _ := json.Marshal(msg.Badges)
			batch.Queue(q,
				ptr(msg.ID), ptr(msg.Channel), ptr(msg.UserID), ptr(msg.Username), ptr(msg.DisplayName), ptr(msg.Text), badgesJSON, ptr(msg.Color),
				boolPtr(msg.IsMod), boolPtr(msg.IsSubscriber), intPtr(msg.Bits), msg.SentAt.UTC(),
			)
			pending++
			if pending >= b.config.MaxBatch {
				flush()
			}
		}
	}
}

func ptr[T any](v T) *T    { return &v }
func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func newBatcher(ctx context.Context, sender batchSender, cfg BatchConfig) *Batcher {
	b := &Batcher{
		input:  make(chan model.ChatMessage, cfg.ChanBuffer),
		config: cfg,
		sender: sender,
	}

	go b.run(ctx)

	return b
}
