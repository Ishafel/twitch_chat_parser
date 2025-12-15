package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	twitch "github.com/gempir/go-twitch-irc/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mustEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("ENV %s is required", key)
	}
	return v
}

// Параметры батчинга/логирования
const (
	maxBatch      = 100                     // сколько сообщений копить перед принудительным флашем
	flushEvery    = 1500 * time.Millisecond // флаш по таймеру (near-realtime)
	chanBuffer    = 4096                    // буфер очереди сообщений
	statsLogEvery = 5 * time.Minute         // как часто логировать статистику
	flushTimeout  = 5 * time.Second         // максимальное время на один флаш в БД
)

type row struct {
	id, ch, uid, uname, dname, txt, color *string
	badges                                []byte
	isMod, isSub                          *bool
	bits                                  *int
	sentAt                                time.Time
}

// батчер: собирает INSERT'ы в pgx.Batch и флашит пачкой,
// плюс логирует статистику по вставкам.
func startBatchWriter(ctx context.Context, pool *pgxpool.Pool) chan<- row {
	in := make(chan row, chanBuffer)

	go func() {
		flushTicker := time.NewTicker(flushEvery)
		statsTicker := time.NewTicker(statsLogEvery)
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

			// отдельный контекст с таймаутом, чтобы не зависнуть навечно на БД
			dbCtx, cancel := context.WithTimeout(context.Background(), flushTimeout)
			defer cancel()

			br := pool.SendBatch(dbCtx, batch)
			if err := br.Close(); err != nil {
				log.Printf("batch flush error: %v", err)
				// при таймауте/ошибке считаем, что эти pending логически "обработаны"
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
				log.Printf("batch writer: context cancelled, total inserted rows = %d", totalInserted)
				return

			case <-flushTicker.C:
				flush()

			case <-statsTicker.C:
				log.Printf(
					"batch writer: inserted %d rows in last %s (total %d)",
					intervalInserted, statsLogEvery, totalInserted,
				)
				intervalInserted = 0

			case r := <-in:
				batch.Queue(q,
					r.id, r.ch, r.uid, r.uname, r.dname, r.txt, r.badges, r.color,
					r.isMod, r.isSub, r.bits, r.sentAt.UTC(),
				)
				pending++
				if pending >= maxBatch {
					flush()
				}
			}
		}
	}()

	return in
}

// счётчик дропнутых сообщений, если очередь батчера переполнится
var dropped uint64

func main() {
	// ---- env
	username := mustEnv("TWITCH_USERNAME")
	oauth := mustEnv("TWITCH_OAUTH_TOKEN") // должен начинаться с "oauth:"
	channelsCSV := mustEnv("TWITCH_CHANNELS")
	channels := splitAndTrim(channelsCSV)

	pgHost := mustEnv("POSTGRES_HOST")
	pgPort := mustEnv("POSTGRES_PORT")
	pgDB := mustEnv("POSTGRES_DB")
	pgUser := mustEnv("POSTGRES_USER")
	pgPass := mustEnv("POSTGRES_PASSWORD")

	dsn := "postgres://" + pgUser + ":" + pgPass + "@" + pgHost + ":" + pgPort + "/" + pgDB + "?sslmode=disable"

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ---- db pool
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	// ---- батчер
	writer := startBatchWriter(ctx, pool)

	// ---- twitch client
	client := twitch.NewClient(username, oauth)

	client.OnPrivateMessage(func(m twitch.PrivateMessage) {
		badgesJSON, _ := json.Marshal(m.User.Badges)

		sentAt := m.Time
		if sentAt.IsZero() {
			sentAt = time.Now().UTC()
		}

		r := row{
			id:     ptr(m.ID),
			ch:     ptr(strings.TrimPrefix(m.Channel, "#")),
			uid:    ptr(m.User.ID),
			uname:  ptr(m.User.Name),
			dname:  ptr(m.User.DisplayName),
			txt:    ptr(m.Message),
			badges: badgesJSON,
			color:  ptr(m.User.Color),
			isMod:  boolPtr(m.User.Badges["moderator"] > 0 || m.User.Badges["broadcaster"] > 0),
			isSub:  boolPtr(m.User.Badges["subscriber"] > 0),
			bits:   intPtr(m.Bits),
			sentAt: sentAt,
		}

		// неблокирующая отправка: если очередь забита, дропаем и логируем
		select {
		case writer <- r:
			// ок
		default:
			dropped++
			if dropped%100 == 0 {
				log.Printf("batch writer: channel full, dropped %d messages total", dropped)
			}
		}
	})

	client.OnConnect(func() {
		log.Printf("twitch: connected, joining channels: %v", channels)
		for _, ch := range channels {
			if ch == "" {
				continue
			}
			client.Join(ch)
		}
	})

	client.OnReconnectMessage(func(message twitch.ReconnectMessage) {
		log.Printf("twitch: RECONNECT requested by server: %+v", message)
	})

	client.OnNoticeMessage(func(msg twitch.NoticeMessage) {
		channel := normalizeChannel(msg.Channel)
		log.Printf("twitch NOTICE #%s [%s]: %s", channel, msg.MsgID, msg.Message)

		if err := saveNotice(ctx, pool, msg); err != nil {
			log.Printf("twitch NOTICE save failed for #%s: %v", channel, err)
		}
	})

	// go-twitch-irc уже умеет автореконнект с бэкоффом
	go func() {
		if err := client.Connect(); err != nil {
			log.Printf("twitch connect error: %v", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	client.Disconnect()
}

func saveNotice(ctx context.Context, pool *pgxpool.Pool, msg twitch.NoticeMessage) error {
	dbCtx, cancel := context.WithTimeout(ctx, flushTimeout)
	defer cancel()

	noticeAt := noticeTimestamp(msg.Tags)
	tagsJSON, _ := json.Marshal(msg.Tags)

	_, err := pool.Exec(dbCtx, `
insert into channel_notices (
  channel, msg_id, message, tags, notice_at
) values ($1, $2, $3, $4, $5);
`, normalizeChannel(msg.Channel), msg.MsgID, msg.Message, tagsJSON, noticeAt)

	return err
}

// --- helpers ---

func noticeTimestamp(tags map[string]string) time.Time {
	if ts := tags["tmi-sent-ts"]; ts != "" {
		if ms, err := strconv.ParseInt(ts, 10, 64); err == nil {
			return time.UnixMilli(ms).UTC()
		}
	}

	return time.Now().UTC()
}

func normalizeChannel(ch string) string {
	return strings.TrimPrefix(strings.TrimSpace(ch), "#")
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.TrimPrefix(p, "#"))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ptr[T any](v T) *T    { return &v }
func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }
