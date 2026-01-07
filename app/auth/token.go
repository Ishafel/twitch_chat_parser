package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const twitchOAuthTokenURL = "https://id.twitch.tv/oauth2/token"

// GetAppToken запрашивает OAuth токен приложения у Twitch.
func GetAppToken(clientID, clientSecret string) (accessToken string, expiresIn time.Duration, err error) {
	form := url.Values{}
	form.Set("client_id", strings.TrimSpace(clientID))
	form.Set("client_secret", strings.TrimSpace(clientSecret))
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequest(http.MethodPost, twitchOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("twitch oauth: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("twitch oauth: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("twitch oauth: unexpected status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", 0, fmt.Errorf("twitch oauth: decode response: %w", err)
	}

	return payload.AccessToken, time.Duration(payload.ExpiresIn) * time.Second, nil
}
