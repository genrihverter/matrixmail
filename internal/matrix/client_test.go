// Package matrix содержит тесты для Matrix клиента
package matrix

import (
	"context"
	"net/http"
	"testing"
	"time"

	"mail-matrix-server/internal/config"
)

// TestNewClient проверяет создание клиента
func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.MatrixConfig
		wantErr bool
	}{
		{
			name: "валидная конфигурация",
			cfg: config.MatrixConfig{
				HomeserverURL:  "https://matrix.org",
				AccessToken:    "test_token",
				UserID:         "@user:matrix.org",
				RequestTimeout: 30,
			},
			wantErr: false,
		},
		{
			name: "отсутствует homeserver URL",
			cfg: config.MatrixConfig{
				HomeserverURL: "",
				AccessToken:   "test_token",
			},
			wantErr: true,
		},
		{
			name: "отсутствует access token",
			cfg: config.MatrixConfig{
				HomeserverURL: "https://matrix.org",
				AccessToken:   "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

// TestExtractMessageText проверяет извлечение текста сообщения
func TestExtractMessageText(t *testing.T) {
	tests := []struct {
		name  string
		event Event
		want  string
	}{
		{
			name: "текстовое сообщение",
			event: Event{
				Type: "m.room.message",
				Content: map[string]interface{}{
					"msgtype": "m.text",
					"body":    "Привет, мир!",
				},
			},
			want: "Привет, мир!",
		},
		{
			name: "не текстовое сообщение",
			event: Event{
				Type: "m.room.message",
				Content: map[string]interface{}{
					"msgtype": "m.image",
					"body":    "image.jpg",
				},
			},
			want: "",
		},
		{
			name: "не сообщение",
			event: Event{
				Type: "m.room.member",
				Content: map[string]interface{}{
					"membership": "join",
				},
			},
			want: "",
		},
		{
			name: "пустое тело",
			event: Event{
				Type: "m.room.message",
				Content: map[string]interface{}{
					"msgtype": "m.text",
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractMessageText(tt.event)
			if got != tt.want {
				t.Errorf("ExtractMessageText() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsMessageEvent проверяет определение типа события
func TestIsMessageEvent(t *testing.T) {
	tests := []struct {
		name  string
		event Event
		want  bool
	}{
		{
			name: "сообщение",
			event: Event{
				Type: "m.room.message",
			},
			want: true,
		},
		{
			name: "не сообщение",
			event: Event{
				Type: "m.room.member",
			},
			want: false,
		},
		{
			name: "событие комнаты",
			event: Event{
				Type: "m.room.create",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMessageEvent(tt.event)
			if got != tt.want {
				t.Errorf("IsMessageEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSyncTimeout проверяет таймаут синхронизации
func TestSyncTimeout(t *testing.T) {
	client := &Client{
		homeserverURL: "https://invalid.invalid",
		accessToken:   "test",
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.Sync(ctx, "")
	if err == nil {
		t.Error("Sync() должен вернуть ошибку для невалидного сервера")
	}
}
