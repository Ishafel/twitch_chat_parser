package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"twitch-chat-logger/model"
)

type stubSender struct {
	mu      sync.Mutex
	batches [][]*pgx.QueuedQuery
}

type stubBatchResults struct{}

func (s *stubSender) SendBatch(_ context.Context, b *pgx.Batch) pgx.BatchResults {
	s.mu.Lock()
	defer s.mu.Unlock()

	copyQueries := append([]*pgx.QueuedQuery(nil), b.QueuedQueries...)
	s.batches = append(s.batches, copyQueries)
	return &stubBatchResults{}
}

func (s *stubBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (s *stubBatchResults) Query() (pgx.Rows, error)         { return nil, nil }
func (s *stubBatchResults) QueryRow() pgx.Row                { return nil }
func (s *stubBatchResults) Close() error                     { return nil }

func TestBatcherFlushesOnMaxBatch(t *testing.T) {
	sender := &stubSender{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batcher := newBatcher(ctx, sender, BatchConfig{
		MaxBatch:      2,
		FlushEvery:    time.Hour,
		ChanBuffer:    10,
		StatsLogEvery: time.Hour,
		FlushTimeout:  time.Second,
	})

	msg := model.ChatMessage{ID: "1", Channel: "ch", UserID: "u", Username: "name", DisplayName: "disp", Text: "hi", SentAt: time.Now()}
	batcher.Enqueue(msg)
	batcher.Enqueue(msg)

	waitForBatches(t, sender, 1)
}

func TestBatcherFlushesOnTimer(t *testing.T) {
	sender := &stubSender{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batcher := newBatcher(ctx, sender, BatchConfig{
		MaxBatch:      10,
		FlushEvery:    50 * time.Millisecond,
		ChanBuffer:    10,
		StatsLogEvery: time.Hour,
		FlushTimeout:  time.Second,
	})

	msg := model.ChatMessage{ID: "2", Channel: "ch", UserID: "u", Username: "name", DisplayName: "disp", Text: "hello", SentAt: time.Now()}
	batcher.Enqueue(msg)

	waitForBatches(t, sender, 1)
}

func waitForBatches(t *testing.T, sender *stubSender, expected int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sender.mu.Lock()
		count := len(sender.batches)
		sender.mu.Unlock()
		if count >= expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected at least %d batches, got %d", expected, len(sender.batches))
}
