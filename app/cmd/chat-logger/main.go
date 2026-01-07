package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"twitch-chat-logger/config"
	"twitch-chat-logger/service"
	"twitch-chat-logger/storage"
	"twitch-chat-logger/twitch"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		log.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	batcher := storage.NewBatcher(ctx, pool, storage.BatchConfig{
		MaxBatch:      cfg.Batch.MaxBatch,
		FlushEvery:    cfg.Batch.FlushEvery,
		ChanBuffer:    cfg.Batch.ChanBuffer,
		StatsLogEvery: cfg.Batch.StatsLogEvery,
		FlushTimeout:  cfg.Batch.FlushTimeout,
	})

	handler := service.NewHandler(batcher, pool, cfg.Batch.FlushTimeout)
	client := twitch.NewClient(cfg.Twitch, handler)
	srv := service.New(client)

	if err := srv.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("service run failed: %v", err)
	}

	log.Println("shutting down...")
}
