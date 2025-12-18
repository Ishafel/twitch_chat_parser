package model

import "time"

// ChatMessage — нормализованная модель сообщения чата Twitch.
type ChatMessage struct {
	ID           string
	Channel      string
	UserID       string
	Username     string
	DisplayName  string
	Text         string
	Badges       map[string]int
	Color        string
	IsMod        bool
	IsSubscriber bool
	Bits         int
	SentAt       time.Time
}

// Notice описывает notice-событие, полученное от Twitch.
type Notice struct {
	Channel  string
	ID       string
	Message  string
	Tags     map[string]string
	NoticeAt time.Time
}
