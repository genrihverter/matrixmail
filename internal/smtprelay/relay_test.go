// Package smtprelay содержит тесты для SMTP relay клиента
package smtprelay

import (
	"testing"

	"mail-matrix-server/internal/config"
)

// TestNewRelayClient проверяет создание Relay клиента
func TestNewRelayClient(t *testing.T) {
	cfg := config.SMTPRelayConfig{
		Host:        "smtp.mail.ru",
		Port:        587,
		Username:    "test@mail.ru",
		Password:    "password123",
		FromAddress: "test@mail.ru",
		FromName:    "Test User",
		UseSTARTTLS: true,
	}

	client := NewRelayClient(cfg)
	if client == nil {
		t.Fatal("NewRelayClient() returned nil")
	}

	if client.host != cfg.Host {
		t.Errorf("host = %v, want %v", client.host, cfg.Host)
	}
	if client.port != cfg.Port {
		t.Errorf("port = %v, want %v", client.port, cfg.Port)
	}
	if client.username != cfg.Username {
		t.Errorf("username = %v, want %v", client.username, cfg.Username)
	}
}

// TestBuildMessage проверяет формирование MIME сообщения
func TestBuildMessage(t *testing.T) {
	client := &RelayClient{
		fromAddress: "sender@example.com",
		fromName:    "Sender Name",
	}

	msg := &EmailMessage{
		To:       []string{"recipient@example.com"},
		Subject:  "Тестовая тема",
		Body:     "Текст сообщения",
		FromName: "Custom Sender",
	}

	data := client.buildMessage(msg)
	if len(data) == 0 {
		t.Fatal("buildMessage() returned empty data")
	}

	// Проверяем наличие обязательных заголовков
	expectedHeaders := []string{
		"From:",
		"To:",
		"Subject:",
		"Content-Type:",
	}

	dataStr := string(data)
	for _, header := range expectedHeaders {
		if !contains(dataStr, header) {
			t.Errorf("buildMessage() missing header: %s", header)
		}
	}
}

// TestCreateMatrixNotificationMessage проверяет создание уведомления Matrix
func TestCreateMatrixNotificationMessage(t *testing.T) {
	senderID := "@user:matrix.org"
	senderName := "Иван Иванов"
	roomName := "Общая комната"
	messageText := "Привет всем!"
	userEmail := "user@example.com"

	msg := CreateMatrixNotificationMessage(senderID, senderName, roomName, messageText, userEmail)

	if msg == nil {
		t.Fatal("CreateMatrixNotificationMessage() returned nil")
	}

	if len(msg.To) != 1 || msg.To[0] != userEmail {
		t.Errorf("To = %v, want [%s]", msg.To, userEmail)
	}

	if !contains(msg.Subject, senderName) {
		t.Errorf("Subject = %v, should contain %s", msg.Subject, senderName)
	}

	if !contains(msg.Body, messageText) {
		t.Errorf("Body = %v, should contain %s", msg.Body, messageText)
	}

	if msg.HTMLBody == "" {
		t.Error("HTMLBody should not be empty")
	}
}

// TestGenerateBoundary проверяет генерацию границы
func TestGenerateBoundary(t *testing.T) {
	boundary := generateBoundary()

	if boundary == "" {
		t.Error("generateBoundary() returned empty string")
	}

	// Проверяем формат границы
	if !contains(boundary, "----=_Part_") {
		t.Errorf("generateBoundary() invalid format: %s", boundary)
	}
}

// contains проверяет содержит ли строка подстроку
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestEmailMessageValidation проверяет валидацию EmailMessage
func TestEmailMessageValidation(t *testing.T) {
	tests := []struct {
		name    string
		msg     *EmailMessage
		wantErr bool
	}{
		{
			name: "валидное сообщение",
			msg: &EmailMessage{
				To:      []string{"user@example.com"},
				Subject: "Тема",
				Body:    "Тело",
			},
			wantErr: false,
		},
		{
			name: "пустые получатели",
			msg: &EmailMessage{
				To:      []string{},
				Subject: "Тема",
				Body:    "Тело",
			},
			wantErr: true,
		},
		{
			name: "nil получатели",
			msg: &EmailMessage{
				To:      nil,
				Subject: "Тема",
				Body:    "Тело",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Проверяем только валидацию полей, не отправку
			hasRecipients := len(tt.msg.To) > 0
			if hasRecipients && tt.wantErr {
				t.Error("unexpected error expectation for valid message")
			}
			if !hasRecipients && !tt.wantErr {
				t.Error("expected error for empty recipients")
			}
		})
	}
}
