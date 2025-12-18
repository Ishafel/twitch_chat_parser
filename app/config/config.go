package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config агрегирует значения конфигурации из переменных окружения.
type Config struct {
	Twitch   TwitchConfig
	Postgres PostgresConfig
	Batch    BatchConfig
}

// TwitchConfig содержит учётные данные и каналы для Twitch IRC клиента.
type TwitchConfig struct {
	Username   string
	OAuthToken string
	Channels   []string
}

// PostgresConfig хранит параметры подключения к пулу базы данных.
type PostgresConfig struct {
	Host     string
	Port     string
	DB       string
	User     string
	Password string
}

// DSN собирает строку подключения для pgx/pgxpool.
func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", p.User, p.Password, p.Host, p.Port, p.DB)
}

// BatchConfig задаёт параметры батчинга и флашей при записи чатов.
type BatchConfig struct {
	MaxBatch      int
	FlushEvery    time.Duration
	ChanBuffer    int
	StatsLogEvery time.Duration
	FlushTimeout  time.Duration
}

// Load читает переменные окружения и возвращает валидированную Config.
func Load() (Config, error) {
	twitchChannels := splitAndTrim(os.Getenv("TWITCH_CHANNELS"))

	cfg := Config{
		Twitch: TwitchConfig{
			Username:   strings.TrimSpace(os.Getenv("TWITCH_USERNAME")),
			OAuthToken: strings.TrimSpace(os.Getenv("TWITCH_OAUTH_TOKEN")),
			Channels:   twitchChannels,
		},
		Postgres: PostgresConfig{
			Host:     strings.TrimSpace(os.Getenv("POSTGRES_HOST")),
			Port:     strings.TrimSpace(os.Getenv("POSTGRES_PORT")),
			DB:       strings.TrimSpace(os.Getenv("POSTGRES_DB")),
			User:     strings.TrimSpace(os.Getenv("POSTGRES_USER")),
			Password: strings.TrimSpace(os.Getenv("POSTGRES_PASSWORD")),
		},
		Batch: BatchConfig{
			MaxBatch:      100,
			FlushEvery:    1500 * time.Millisecond,
			ChanBuffer:    4096,
			StatsLogEvery: 5 * time.Minute,
			FlushTimeout:  5 * time.Second,
		},
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if c.Twitch.Username == "" {
		return fmt.Errorf("требуется TWITCH_USERNAME")
	}
	if c.Twitch.OAuthToken == "" {
		return fmt.Errorf("требуется TWITCH_OAUTH_TOKEN")
	}
	if len(c.Twitch.Channels) == 0 {
		return fmt.Errorf("требуется TWITCH_CHANNELS")
	}

	if c.Postgres.Host == "" {
		return fmt.Errorf("требуется POSTGRES_HOST")
	}
	if c.Postgres.Port == "" {
		return fmt.Errorf("требуется POSTGRES_PORT")
	}
	if c.Postgres.DB == "" {
		return fmt.Errorf("требуется POSTGRES_DB")
	}
	if c.Postgres.User == "" {
		return fmt.Errorf("требуется POSTGRES_USER")
	}
	if c.Postgres.Password == "" {
		return fmt.Errorf("требуется POSTGRES_PASSWORD")
	}

	if c.Batch.MaxBatch <= 0 {
		return fmt.Errorf("Batch.MaxBatch должен быть больше нуля")
	}
	if c.Batch.FlushEvery <= 0 {
		return fmt.Errorf("Batch.FlushEvery должен быть больше нуля")
	}
	if c.Batch.ChanBuffer <= 0 {
		return fmt.Errorf("Batch.ChanBuffer должен быть больше нуля")
	}
	if c.Batch.StatsLogEvery <= 0 {
		return fmt.Errorf("Batch.StatsLogEvery должен быть больше нуля")
	}
	if c.Batch.FlushTimeout <= 0 {
		return fmt.Errorf("Batch.FlushTimeout должен быть больше нуля")
	}

	return nil
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
