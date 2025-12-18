package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"twitch-chat-logger/model"
)

// SaveNotice сохраняет notice-событие в базе с учётом заданного таймаута.
func SaveNotice(ctx context.Context, pool *pgxpool.Pool, notice model.Notice, timeout time.Duration) error {
	dbCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tagsJSON, _ := json.Marshal(notice.Tags)

	_, err := pool.Exec(dbCtx, `
insert into channel_notices (
  channel, msg_id, message, tags, notice_at
) values ($1, $2, $3, $4, $5);
`, notice.Channel, notice.ID, notice.Message, tagsJSON, notice.NoticeAt)

	return err
}
