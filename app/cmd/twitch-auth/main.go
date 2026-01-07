package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"twitch-chat-logger/auth"
	"twitch-chat-logger/tokens"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "app" {
		fmt.Fprintln(os.Stderr, "usage: twitch-auth app")
		os.Exit(1)
	}

	clientID := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_ID"))
	if clientID == "" {
		log.Fatal("TWITCH_CLIENT_ID is required")
	}

	clientSecret := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_SECRET"))
	if clientSecret == "" {
		log.Fatal("TWITCH_CLIENT_SECRET is required")
	}

	store := tokens.FileTokenStore{}
	manager := tokens.NewAppTokenManager(store, func() (string, time.Duration, error) {
		return auth.GetAppToken(clientID, clientSecret)
	})

	ctx := context.Background()
	token, err := manager.Get(ctx)
	if err != nil {
		log.Fatalf("get app token: %v", err)
	}

	if err := store.SaveAppToken(token); err != nil {
		log.Fatalf("save app token: %v", err)
	}

	fmt.Printf("ok, expires at %s\n", token.ExpiresAt.Format(time.RFC3339))
}
