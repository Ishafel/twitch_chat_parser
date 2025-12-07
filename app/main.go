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
	"github.com/jackc/pgx/v5/pgxpool"
)

func mustEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("ENV %s is required", key)
	}
	return v
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

	// ---- twitch client
	client := twitch.NewClient(username, oauth)
	client.OnPrivateMessage(func(m twitch.PrivateMessage) {
		// badges -> jsonb
		badgesJSON, _ := json.Marshal(m.User.Badges)

		// Insert; ON CONFLICT ignore duplicate message_id
		const q = `
insert into chat_messages (
  message_id, channel, user_id, username, display_name, text, badges, color,
  is_mod, is_subscriber, bits, sent_at
) values (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12
)
on conflict (message_id) do nothing;`

		sentAt := m.Time
		if sentAt.IsZero() {
			sentAt = time.Now().UTC()
		}

		_, err := pool.Exec(ctx, q,
			nullable(m.ID),
			strings.TrimPrefix(m.Channel, "#"),
			nullable(m.User.ID),
			nullable(m.User.Name),
			nullable(m.User.DisplayName),
			m.Message,
			badgesJSON,
			nullable(m.User.Color),
			boolPtr(m.User.Badges["moderator"] > 0 || m.User.Badges["broadcaster"] > 0),
			boolPtr(m.User.Badges["subscriber"] > 0),
			intPtr(m.Bits),
			sentAt.UTC(),
		)
		if err != nil {
			log.Printf("insert failed: %v", err)
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

func nullable[T any](v T) any { return v }
func boolPtr(b bool) *bool    { return &b }
func intPtr(i int) *int       { return &i }
