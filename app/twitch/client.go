package twitch

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	twitchirc "github.com/gempir/go-twitch-irc/v4"

	"twitch-chat-logger/config"
	"twitch-chat-logger/model"
)

// Handler принимает Twitch-события, преобразованные в доменные модели.
type Handler interface {
	HandleChat(context.Context, model.ChatMessage)
	HandleNotice(context.Context, model.Notice)
}

// Client оборачивает go-twitch-irc и настраивает обработчики.
type Client struct {
	client   *twitchirc.Client
	handler  Handler
	channels []string
	baseCtx  context.Context
}

// NewClient инициализирует IRC-клиент и регистрирует колбэки.
func NewClient(cfg config.TwitchConfig, handler Handler) *Client {
	client := twitchirc.NewClient(cfg.Username, cfg.OAuthToken)

	c := &Client{
		client:   client,
		handler:  handler,
		channels: cfg.Channels,
	}

	client.OnPrivateMessage(func(m twitchirc.PrivateMessage) {
		c.handler.HandleChat(c.context(), toChatMessage(m))
	})

	client.OnConnect(func() {
		log.Printf("twitch: подключено, подписка на каналы: %v", cfg.Channels)
		for _, ch := range cfg.Channels {
			if ch == "" {
				continue
			}
			client.Join(ch)
		}
	})

	client.OnReconnectMessage(func(message twitchirc.ReconnectMessage) {
		log.Printf("twitch: сервер запросил RECONNECT: %+v", message)
	})

	client.OnNoticeMessage(func(msg twitchirc.NoticeMessage) {
		c.handler.HandleNotice(c.context(), toNotice(msg))
	})

	return c
}

// Run подключает клиента и блокируется до отмены контекста или ошибки.
func (c *Client) Run(ctx context.Context) error {
	c.baseCtx = ctx
	errCh := make(chan error, 1)

	go func() {
		errCh <- c.client.Connect()
	}()

	select {
	case <-ctx.Done():
		c.client.Disconnect()
		<-errCh
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func toChatMessage(m twitchirc.PrivateMessage) model.ChatMessage {
	badges := make(map[string]int, len(m.User.Badges))
	for k, v := range m.User.Badges {
		badges[k] = v
	}

	sentAt := m.Time
	if sentAt.IsZero() {
		sentAt = time.Now().UTC()
	}

	return model.ChatMessage{
		ID:           m.ID,
		Channel:      normalizeChannel(m.Channel),
		UserID:       m.User.ID,
		Username:     m.User.Name,
		DisplayName:  m.User.DisplayName,
		Text:         m.Message,
		Badges:       badges,
		Color:        m.User.Color,
		IsMod:        m.User.Badges["moderator"] > 0 || m.User.Badges["broadcaster"] > 0,
		IsSubscriber: m.User.Badges["subscriber"] > 0,
		Bits:         m.Bits,
		SentAt:       sentAt,
	}
}

func toNotice(msg twitchirc.NoticeMessage) model.Notice {
	return model.Notice{
		Channel:  normalizeChannel(msg.Channel),
		ID:       msg.MsgID,
		Message:  msg.Message,
		Tags:     msg.Tags,
		NoticeAt: noticeTimestamp(msg.Tags),
	}
}

func noticeTimestamp(tags map[string]string) time.Time {
	if ts := tags["tmi-sent-ts"]; ts != "" {
		if ms, err := strconv.ParseInt(ts, 10, 64); err == nil {
			return time.UnixMilli(ms).UTC()
		}
	}

	return time.Now().UTC()
}

func normalizeChannel(ch string) string {
	return strings.TrimPrefix(strings.TrimSpace(ch), "#")
}

func (c *Client) context() context.Context {
	if c.baseCtx != nil {
		return c.baseCtx
	}
	return context.Background()
}
