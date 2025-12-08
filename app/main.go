package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
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

const (
	maxBatch   = 100                     // флаш по размеру (при всплесках)
	flushEvery = 1500 * time.Millisecond // флаш по таймеру (near-realtime)
	chanBuffer = 4096                    // буфер очереди сообщений
)

type row struct {
	id, ch, uid, uname, dname, txt, color *string
	badges                                 []byte
	isMod, isSub                           *bool
	bits                                   *int
	sentAt                                 time.Time
}

// батчер: собирает INSERT'ы в pgx.Batch и флашит пачкой
func startBatchWriter(ctx context.Context, pool *pgxpool.Pool) chan<- row {
	in := make(chan row, chanBuffer)

	go func() {
		ticker := time.NewTicker(flushEvery)
		defer ticker.Stop()

		var (
			batch   = &pgx.Batch{}
			pending = 0
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
			br := pool.SendBatch(ctx, batch)
			if err := br.Close(); err != nil {
				log.Printf("batch flush error: %v", err)
			}
			batch = &pgx.Batch{}
			pending = 0
		}

		for {
			select {
			case <-ctx.Done():
				flush()
				return
			case <-ticker.C:
				flush()
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

	// запустим батчер
	writer := startBatchWriter(ctx, pool)

	// ---- twitch client
	client := twitch.NewClient(username, oauth)

	client.OnPrivateMessage(func(m twitch.PrivateMessage) {
		badgesJSON, _ := json.Marshal(m.User.Badges)

		sentAt := m.Time
		if sentAt.IsZero() {
			sentAt = time.Now().UTC()
		}

		// отправляем в очередь; запись попадёт в следующий батч
		writer <- row{
			id:    ptr(m.ID),
			ch:    ptr(strings.TrimPrefix(m.Channel, "#")),
			uid:   ptr(m.User.ID),
			uname: ptr(m.User.Name),
			dname: ptr(m.User.DisplayName),
			txt:   ptr(m.Message),
			badges: badgesJSON,
			color:  ptr(m.User.Color),
			isMod:  boolPtr(m.User.Badges["moderator"] > 0 || m.User.Badges["broadcaster"] > 0),
			isSub:  boolPtr(m.User.Badges["subscriber"] > 0),
			bits:   intPtr(m.Bits),
			sentAt: sentAt,
		}
	})

	client.OnConnect(func() {
		log.Printf("connected. joining: %v", channels)
		for _, ch := range channels {
			if ch == "" {
				continue
			}
			client.Join(ch)
		}
	})

	client.OnReconnectMessage(func(message twitch.ReconnectMessage) {
		log.Printf("server asked to reconnect: %+v", message)
	})

	// go-twitch-irc уже делает автореконнект
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

// --- helpers ---

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

func ptr[T any](v T) *T        { return &v }
func boolPtr(b bool) *bool     { return &b }
func intPtr(i int) *int        { return &i }
