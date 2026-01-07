package tokens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const TOKEN_FILE = ".secrets/twitch_tokens.json"

// FileTokenStore сохраняет токены в JSON файле.
type FileTokenStore struct {
	Path string
}

type fileToken struct {
	Access    string `json:"access"`
	ExpiresAt string `json:"expires_at"`
}

func (store FileTokenStore) tokenPath() string {
	if strings.TrimSpace(store.Path) == "" {
		return TOKEN_FILE
	}
	return store.Path
}

// LoadAppToken загружает OAuth токен приложения из JSON файла.
func (store FileTokenStore) LoadAppToken() (*Token, error) {
	path := store.tokenPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load app token: read file: %w", err)
	}

	var payload fileToken
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("load app token: decode json: %w", err)
	}

	expiresAt, err := time.Parse(time.RFC3339, payload.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("load app token: parse expires_at: %w", err)
	}

	return &Token{
		Access:    payload.Access,
		ExpiresAt: expiresAt,
	}, nil
}

// SaveAppToken сохраняет OAuth токен приложения в JSON файл.
func (store FileTokenStore) SaveAppToken(token Token) error {
	path := store.tokenPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("save app token: create dir: %w", err)
	}

	payload := fileToken{
		Access:    token.Access,
		ExpiresAt: token.ExpiresAt.Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("save app token: encode json: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("save app token: write file: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("save app token: chmod file: %w", err)
	}

	return nil
}
