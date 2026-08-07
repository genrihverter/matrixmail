// Package notifier предоставляет функционал для отправки уведомлений из Matrix в Email
package notifier

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"mail-matrix-server/internal/config"
	"mail-matrix-server/internal/matrix"
	"mail-matrix-server/internal/smtprelay"
)

// Notifier управляет отправкой уведомлений из Matrix в Email
type Notifier struct {
	matrixClient   *matrix.Client
	smtpClient     *smtprelay.RelayClient
	mappings       map[string]config.MatrixUserMapping // matrix_user_id -> mapping
	roomNames      map[string]string                   // room_id -> room_name
	userNames      map[string]string                   // user_id -> display name
	cfg            config.MatrixConfig
	reconnectDelay time.Duration
	mu             sync.RWMutex
	stopChan       chan struct{}
	isRunning      bool
}

// NewNotifier создает новый нотификатор
func NewNotifier(matrixCfg config.MatrixConfig, smtpCfg config.SMTPRelayConfig, mappings []config.MatrixUserMapping) (*Notifier, error) {
	// Создаем Matrix клиент
	matrixClient, err := matrix.NewClient(matrixCfg)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Matrix клиента: %w", err)
	}

	// Создаем SMTP клиент
	smtpClient := smtprelay.NewRelayClient(smtpCfg)

	// Строим маппинг пользователей
	mappingMap := make(map[string]config.MatrixUserMapping)
	for _, m := range mappings {
		if m.Enabled {
			mappingMap[m.MatrixUserID] = m
		}
	}

	reconnectDelay := time.Duration(matrixCfg.ReconnectDelay) * time.Second
	if reconnectDelay == 0 {
		reconnectDelay = 10 * time.Second
	}

	return &Notifier{
		matrixClient:   matrixClient,
		smtpClient:     smtpClient,
		mappings:       mappingMap,
		roomNames:      make(map[string]string),
		userNames:      make(map[string]string),
		cfg:            matrixCfg,
		reconnectDelay: reconnectDelay,
		stopChan:       make(chan struct{}),
	}, nil
}

// Start запускает прослушивание событий Matrix
func (n *Notifier) Start() error {
	n.mu.Lock()
	if n.isRunning {
		n.mu.Unlock()
		return fmt.Errorf("нотификатор уже запущен")
	}
	n.isRunning = true
	n.mu.Unlock()

	go n.runSyncLoop()
	return nil
}

// Stop останавливает нотификатор
func (n *Notifier) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.isRunning {
		return nil
	}

	close(n.stopChan)
	n.isRunning = false
	return nil
}

// runSyncLoop выполняет цикл синхронизации с Matrix
func (n *Notifier) runSyncLoop() {
	var sinceToken string

	for {
		select {
		case <-n.stopChan:
			log.Println("[Notifier] Остановка синхронизации")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		
		syncResp, err := n.matrixClient.Sync(ctx, sinceToken)
		cancel()

		if err != nil {
			log.Printf("[Notifier] Ошибка синхронизации: %v. Повтор через %v", err, n.reconnectDelay)
			time.Sleep(n.reconnectDelay)
			continue
		}

		// Обновляем токен для следующей синхронизации
		sinceToken = syncResp.NextBatch

		// Обрабатываем события из комнат
		n.processRooms(syncResp.Rooms.Join)
	}
}

// processRooms обрабатывает события из комнат
func (n *Notifier) processRooms(joinedRooms map[string]matrix.JoinedRoom) {
	for roomID, joinedRoom := range joinedRooms {
		// Получаем имя комнаты (кэшируем)
		roomName := n.getRoomName(roomID)

		// Обрабатываем события timeline
		for _, event := range joinedRoom.Timeline.Events {
			// Пропускаем не сообщения
			if !matrix.IsMessageEvent(event) {
				continue
			}

			// Извлекаем текст сообщения
			messageText := matrix.ExtractMessageText(event)
			if messageText == "" {
				continue
			}

			// Получаем имя отправителя
			senderName := n.getUserName(event.Sender)

			// Отправляем уведомления подписчикам
			n.sendNotifications(event.Sender, senderName, roomID, roomName, messageText)
		}
	}
}

// sendNotifications отправляет уведомления всем подписчикам
func (n *Notifier) sendNotifications(senderID, senderName, roomID, roomName, messageText string) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for matrixUserID, mapping := range n.mappings {
		// Не отправляем уведомление самому отправителю
		if matrixUserID == senderID {
			continue
		}

		// Проверяем подписку на комнату (если указаны конкретные комнаты)
		if len(mapping.RoomIDs) > 0 {
			found := false
			for _, subscribedRoomID := range mapping.RoomIDs {
				if subscribedRoomID == roomID || subscribedRoomID == "*" {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Создаем и отправляем email
		msg := smtprelay.CreateMatrixNotificationMessage(
			senderID,
			senderName,
			roomName,
			messageText,
			mapping.Email,
		)

		// Отправляем асинхронно для производительности
		go func(userID string, email string) {
			if err := n.smtpClient.SendEmail(msg); err != nil {
				log.Printf("[Notifier] Ошибка отправки email пользователю %s (%s): %v", 
					userID, email, err)
			} else {
				log.Printf("[Notifier] Уведомление отправлено пользователю %s (%s)", 
					userID, email)
			}
		}(matrixUserID, mapping.Email)
	}
}

// getRoomName получает имя комнаты (с кэшированием)
func (n *Notifier) getRoomName(roomID string) string {
	n.mu.RLock()
	if name, ok := n.roomNames[roomID]; ok {
		n.mu.RUnlock()
		return name
	}
	n.mu.RUnlock()

	// В реальной реализации можно запросить у Matrix API
	// Здесь используем заглушку - ID комнаты
	name := roomID
	
	n.mu.Lock()
	n.roomNames[roomID] = name
	n.mu.Unlock()

	return name
}

// GetUserName получает отображаемое имя пользователя
func (n *Notifier) getUserName(userID string) string {
	n.mu.RLock()
	if name, ok := n.userNames[userID]; ok {
		n.mu.RUnlock()
		return name
	}
	n.mu.RUnlock()

	// Пытаемся получить имя через Matrix API
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	name, err := n.matrixClient.GetDisplayName(ctx, userID)
	cancel()

	if err != nil || name == "" {
		// Используем часть userID до :
		parts := strings.Split(userID, ":")
		name = parts[0]
		if strings.HasPrefix(name, "@") {
			name = name[1:]
		}
	}

	n.mu.Lock()
	n.userNames[userID] = name
	n.mu.Unlock()

	return name
}

// AddMapping добавляет новое соответствие пользователя
func (n *Notifier) AddMapping(mapping config.MatrixUserMapping) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if mapping.Enabled {
		n.mappings[mapping.MatrixUserID] = mapping
	}
}

// RemoveMapping удаляет соответствие пользователя
func (n *Notifier) RemoveMapping(matrixUserID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.mappings, matrixUserID)
}

// ValidateSMTPConnection проверяет подключение к SMTP серверу
func (n *Notifier) ValidateSMTPConnection() error {
	return n.smtpClient.ValidateConnection()
}

// IsRunning возвращает статус работы нотификатора
func (n *Notifier) IsRunning() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.isRunning
}
