package service

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"twitch-chat-logger/model"
	"twitch-chat-logger/storage"
	"twitch-chat-logger/twitch"
)

// Service управляет жизненным циклом Twitch клиента и записью в хранилище.
type Service struct {
	client *twitch.Client
}

// New создаёт Service с уже собранным Twitch клиентом.
func New(client *twitch.Client) *Service {
	return &Service{client: client}
}

// Run подключает Twitch клиент и блокируется до отмены контекста или ошибки.
func (s *Service) Run(ctx context.Context) error {
	return s.client.Run(ctx)
}

// Handler реализует twitch.Handler и перенаправляет события в хранилище.
type Handler struct {
	batcher      *storage.Batcher
	pool         *pgxpool.Pool
	flushTimeout time.Duration
}

// NewHandler собирает Handler, используемый Twitch колбэками.
func NewHandler(batcher *storage.Batcher, pool *pgxpool.Pool, flushTimeout time.Duration) *Handler {
	return &Handler{batcher: batcher, pool: pool, flushTimeout: flushTimeout}
}

// HandleChat помещает сообщения чата в очередь батчера.
func (h *Handler) HandleChat(_ context.Context, msg model.ChatMessage) {
	if ok := h.batcher.Enqueue(msg); !ok {
		log.Printf("батчер: сообщение для канала %s отброшено", msg.Channel)
	}
}

// HandleNotice сохраняет notice-событие напрямую через пул БД.
func (h *Handler) HandleNotice(ctx context.Context, notice model.Notice) {
	if err := storage.SaveNotice(ctx, h.pool, notice, h.flushTimeout); err != nil {
		log.Printf("ошибка сохранения NOTICE для #%s: %v", notice.Channel, err)
	}
}
