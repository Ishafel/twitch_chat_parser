package tokens

import "time"

// Token описывает OAuth токен приложения.
type Token struct {
	Access    string
	ExpiresAt time.Time
}

// TokenStore описывает хранилище токенов приложения.
type TokenStore interface {
	LoadAppToken() (*Token, error)
	SaveAppToken(Token) error
}
