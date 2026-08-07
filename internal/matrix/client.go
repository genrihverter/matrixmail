// Package matrix предоставляет клиент для подключения к Matrix API
package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"mail-matrix-server/internal/config"
)

// Client - клиент для работы с Matrix API
type Client struct {
	homeserverURL string
	accessToken   string
	userID        string
	httpClient    *http.Client
	mu            sync.RWMutex
}

// Event представляет событие из комнаты Matrix
type Event struct {
	EventID     string                 `json:"event_id"`
	RoomID      string                 `json:"room_id"`
	Sender      string                 `json:"sender"`
	Type        string                 `json:"type"`
	Timestamp   int64                  `json:"origin_server_ts"`
	Content     map[string]interface{} `json:"content"`
	Unsigned    map[string]interface{} `json:"unsigned,omitempty"`
	StateKey    *string                `json:"state_key,omitempty"`
	Redacts     string                 `json:"redacts,omitempty"`
}

// Room представляет комнату Matrix
type Room struct {
	RoomID      string                 `json:"room_id"`
	Name        string                 `json:"name,omitempty"`
	Members     []string               `json:"members,omitempty"`
	LastEventID string                 `json:"last_event_id,omitempty"`
}

// SyncResponse представляет ответ от /sync API
type SyncResponse struct {
	NextBatch string           `json:"next_batch"`
	Rooms     RoomsResponse    `json:"rooms"`
}

// RoomsResponse содержит информацию о комнатах
type RoomsResponse struct {
	Join map[string]JoinedRoom `json:"join"`
}

// JoinedRoom содержит информацию о присоединенной комнате
type JoinedRoom struct {
	Timeline Timeline `json:"timeline"`
}

// Timeline содержит временную шкалу событий
type Timeline struct {
	Limited bool     `json:"limited"`
	Events  []Event  `json:"events"`
	PrevBatch string `json:"prev_batch,omitempty"`
}

// NewClient создает новый Matrix клиент
func NewClient(cfg config.MatrixConfig) (*Client, error) {
	if cfg.HomeserverURL == "" {
		return nil, fmt.Errorf("homeserver URL не указан")
	}
	if cfg.AccessToken == "" {
		return nil, fmt.Errorf("access token не указан")
	}

	timeout := time.Duration(cfg.RequestTimeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		homeserverURL: cfg.HomeserverURL,
		accessToken:   cfg.AccessToken,
		userID:        cfg.UserID,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// Sync выполняет синхронизацию с сервером Matrix
// Возвращает события из всех комнат и токен для следующей синхронизации
func (c *Client) Sync(ctx context.Context, sinceToken string) (*SyncResponse, error) {
	url := fmt.Sprintf("%s/_matrix/client/v3/sync", c.homeserverURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	query := req.URL.Query()
	if sinceToken != "" {
		query.Set("since", sinceToken)
	}
	query.Set("timeout", "30000") // 30 секунд таймаут на сервере
	req.URL.RawQuery = query.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ошибка Matrix API: статус %d, тело: %s", resp.StatusCode, string(body))
	}

	var syncResp SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	return &syncResp, nil
}

// GetRoomMembers получает список участников комнаты
func (c *Client) GetRoomMembers(ctx context.Context, roomID string) ([]string, error) {
	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/members", c.homeserverURL, roomID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ошибка Matrix API: статус %d, тело: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Chunk []Event `json:"chunk"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	members := make([]string, 0, len(result.Chunk))
	for _, event := range result.Chunk {
		if event.Content["membership"] == "join" && event.StateKey != nil {
			members = append(members, *event.StateKey)
		}
	}

	return members, nil
}

// SendMessage отправляет сообщение в комнату (для тестирования)
func (c *Client) SendMessage(ctx context.Context, roomID, message string) error {
	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message", c.homeserverURL, roomID)

	body := map[string]interface{}{
		"msgtype": "m.text",
		"body":    message,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("ошибка сериализации: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ошибка Matrix API: статус %d, тело: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetDisplayName получает отображаемое имя пользователя
func (c *Client) GetDisplayName(ctx context.Context, userID string) (string, error) {
	url := fmt.Sprintf("%s/_matrix/client/v3/profile/%s/displayname", c.homeserverURL, userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil // Игнорируем ошибку, возвращаем пустое имя
	}

	var result struct {
		DisplayName string `json:"displayname"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil
	}

	return result.DisplayName, nil
}

// ExtractMessageText извлекает текст сообщения из события
func ExtractMessageText(event Event) string {
	if event.Type != "m.room.message" {
		return ""
	}

	content := event.Content
	msgType, ok := content["msgtype"].(string)
	if !ok || msgType != "m.text" {
		return ""
	}

	body, ok := content["body"].(string)
	if !ok {
		return ""
	}

	return body
}

// IsMessageEvent проверяет является ли событие сообщением
func IsMessageEvent(event Event) bool {
	return event.Type == "m.room.message"
}
