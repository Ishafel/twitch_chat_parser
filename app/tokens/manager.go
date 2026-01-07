package tokens

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"
)

// AppTokenFetcher запрашивает токен приложения.
type AppTokenFetcher func() (accessToken string, expiresIn time.Duration, err error)

// AppTokenManager управляет OAuth токеном приложения.
type AppTokenManager struct {
	store    TokenStore
	getToken AppTokenFetcher
	mu       sync.Mutex
}

// NewAppTokenManager создает менеджер токенов приложения.
func NewAppTokenManager(store TokenStore, getToken AppTokenFetcher) *AppTokenManager {
	return &AppTokenManager{
		store:    store,
		getToken: getToken,
	}
}

// Get возвращает OAuth токен приложения, обновляя его при необходимости.
func (manager *AppTokenManager) Get(ctx context.Context) (Token, error) {
	if err := ctx.Err(); err != nil {
		return Token{}, err
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()

	token, err := manager.store.LoadAppToken()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Token{}, err
		}
	}

	if token != nil && !isTokenExpiringSoon(token) {
		return *token, nil
	}

	if err := ctx.Err(); err != nil {
		return Token{}, err
	}

	accessToken, expiresIn, err := manager.getToken()
	if err != nil {
		return Token{}, err
	}

	newToken := Token{
		Access:    accessToken,
		ExpiresAt: time.Now().Add(expiresIn),
	}

	if err := manager.store.SaveAppToken(newToken); err != nil {
		return Token{}, err
	}

	return newToken, nil
}

func isTokenExpiringSoon(token *Token) bool {
	return token.ExpiresAt.Before(time.Now().Add(5 * time.Minute))
}
