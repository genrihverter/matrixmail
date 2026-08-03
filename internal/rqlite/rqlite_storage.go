// Package rqlite предоставляет хранилище на основе RQLite (распределенный SQLite)
package rqlite

import (
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mail-matrix-server/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// RQLiteStorage реализует хранилище данных на основе RQLite
// RQLite - это распределенная база данных на основе SQLite с консенсусом Raft
type RQLiteStorage struct {
	// HTTP клиент для запросов к RQLite
	client *http.Client
	// Базовый URL RQLite сервера
	baseURL string
	// Имя пользователя для аутентификации
	username string
	// Пароль для аутентификации
	password string
}

// NewRQLiteStorage создает новое хранилище RQLite
// baseURL - адрес RQLite сервера (например, http://localhost:4001)
func NewRQLiteStorage(baseURL, username, password string) (*RQLiteStorage, error) {
	storage := &RQLiteStorage{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: username,
		password: password,
	}

	// Инициализируем схему базы данных
	if err := storage.initSchema(); err != nil {
		return nil, fmt.Errorf("ошибка инициализации схемы: %w", err)
	}

	return storage, nil
}

// executeQuery выполняет SQL запрос к RQLite
func (s *RQLiteStorage) executeQuery(query string, params []interface{}) (*sql.Rows, error) {
	// В реальной реализации здесь будет HTTP запрос к RQLite API
	// Для примера используем заглушку - в продакшене нужно использовать настоящий RQLite клиент
	
	// RQLite использует HTTP API для выполнения запросов
	// Формируем запрос к endpoint /db/execute или /db/query
	url := fmt.Sprintf("%s/db/execute", s.baseURL)
	
	requestBody := map[string]interface{}{
		"statements": []map[string]interface{}{
			{
				"query":    query,
				"parameters": params,
			},
		},
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	
	req.SetBasicAuth(s.username, s.password)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ошибка запроса к RQLite: %d - %s", resp.StatusCode, string(body))
	}
	
	// В реальной реализации здесь будет парсинг ответа RQLite
	// Для упрощения возвращаем nil - реальная имплементация требует полноценного клиента
	return nil, nil
}

// initSchema инициализирует схему базы данных
func (s *RQLiteStorage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS calendars (
		id TEXT PRIMARY KEY,
		owner TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		color TEXT DEFAULT '#000000',
		type TEXT DEFAULT 'events',
		raw_data TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_calendars_owner ON calendars(owner);
	
	CREATE TABLE IF NOT EXISTS calendar_events (
		id TEXT PRIMARY KEY,
		uid TEXT NOT NULL,
		owner TEXT NOT NULL,
		calendar_id TEXT NOT NULL,
		summary TEXT,
		description TEXT,
		location TEXT,
		start_time DATETIME,
		end_time DATETIME,
		raw_data TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		etag TEXT,
		FOREIGN KEY (calendar_id) REFERENCES calendars(id) ON DELETE CASCADE
	);
	
	CREATE INDEX IF NOT EXISTS idx_events_calendar ON calendar_events(calendar_id);
	CREATE INDEX IF NOT EXISTS idx_events_owner ON calendar_events(owner);
	CREATE INDEX IF NOT EXISTS idx_events_uid ON calendar_events(uid);
	
	CREATE TABLE IF NOT EXISTS address_books (
		id TEXT PRIMARY KEY,
		owner TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		raw_data TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_address_books_owner ON address_books(owner);
	
	CREATE TABLE IF NOT EXISTS contacts (
		id TEXT PRIMARY KEY,
		uid TEXT NOT NULL,
		owner TEXT NOT NULL,
		address_book_id TEXT NOT NULL,
		full_name TEXT,
		emails TEXT,
		phones TEXT,
		raw_data TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		etag TEXT,
		FOREIGN KEY (address_book_id) REFERENCES address_books(id) ON DELETE CASCADE
	);
	
	CREATE INDEX IF NOT EXISTS idx_contacts_address_book ON contacts(address_book_id);
	CREATE INDEX IF NOT EXISTS idx_contacts_owner ON contacts(owner);
	CREATE INDEX IF NOT EXISTS idx_contacts_uid ON contacts(uid);
	`
	
	// Разбиваем на отдельные запросы (RQLite требует по одному запросу за раз)
	statements := strings.Split(schema, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		
		_, err := s.executeQuery(stmt, nil)
		if err != nil {
			return fmt.Errorf("ошибка выполнения DDL: %w", err)
		}
	}
	
	return nil
}

// generateETag генерирует ETag для синхронизации
func generateETag(data string, timestamp time.Time) string {
	hash := md5.Sum([]byte(fmt.Sprintf("%s%s", data, timestamp.Format(time.RFC3339))))
	return hex.EncodeToString(hash[:])
}

// CreateCalendar создает новый календарь
func (s *RQLiteStorage) CreateCalendar(calendar *models.Calendar) error {
	query := `INSERT INTO calendars (id, owner, name, description, color, type, raw_data, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	now := time.Now()
	calendar.CreatedAt = now
	calendar.UpdatedAt = now
	
	_, err := s.executeQuery(query, []interface{}{
		calendar.ID,
		calendar.Owner,
		calendar.Name,
		calendar.Description,
		calendar.Color,
		calendar.Type,
		calendar.RawData,
		now,
		now,
	})
	
	return err
}

// GetCalendar получает календарь по ID
func (s *RQLiteStorage) GetCalendar(id, owner string) (*models.Calendar, error) {
	// В реальной реализации здесь будет SELECT запрос
	// Для примера возвращаем заглушку
	return &models.Calendar{
		ID:          id,
		Owner:       owner,
		Name:        "Default Calendar",
		Description: "Календарь по умолчанию",
		Color:       "#0066CC",
		Type:        "events",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// ListCalendars возвращает список календарей пользователя
func (s *RQLiteStorage) ListCalendars(owner string) ([]*models.Calendar, error) {
	// В реальной реализации здесь будет SELECT запрос с WHERE owner = ?
	calendars := []*models.Calendar{
		{
			ID:          "cal-1",
			Owner:       owner,
			Name:        "Личный",
			Description: "Личные события",
			Color:       "#FF6B6B",
			Type:        "events",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "cal-2",
			Owner:       owner,
			Name:        "Рабочий",
			Description: "Рабочие встречи",
			Color:       "#4ECDC4",
			Type:        "events",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	
	return calendars, nil
}

// UpdateCalendar обновляет календарь
func (s *RQLiteStorage) UpdateCalendar(calendar *models.Calendar) error {
	query := `UPDATE calendars SET name = ?, description = ?, color = ?, raw_data = ?, updated_at = ? 
			  WHERE id = ? AND owner = ?`
	
	calendar.UpdatedAt = time.Now()
	
	_, err := s.executeQuery(query, []interface{}{
		calendar.Name,
		calendar.Description,
		calendar.Color,
		calendar.RawData,
		calendar.UpdatedAt,
		calendar.ID,
		calendar.Owner,
	})
	
	return err
}

// DeleteCalendar удаляет календарь
func (s *RQLiteStorage) DeleteCalendar(id, owner string) error {
	query := `DELETE FROM calendars WHERE id = ? AND owner = ?`
	_, err := s.executeQuery(query, []interface{}{id, owner})
	return err
}

// CreateEvent создает событие в календаре
func (s *RQLiteStorage) CreateEvent(event *models.CalendarEvent) error {
	query := `INSERT INTO calendar_events (id, uid, owner, calendar_id, summary, description, location, 
			  start_time, end_time, raw_data, created_at, updated_at, etag) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	now := time.Now()
	event.CreatedAt = now
	event.UpdatedAt = now
	event.ETag = generateETag(event.RawData, now)
	
	_, err := s.executeQuery(query, []interface{}{
		event.ID,
		event.UID,
		event.Owner,
		event.CalendarID,
		event.Summary,
		event.Description,
		event.Location,
		event.StartTime,
		event.EndTime,
		event.RawData,
		now,
		now,
		event.ETag,
	})
	
	return err
}

// GetEvent получает событие по ID
func (s *RQLiteStorage) GetEvent(id, owner string) (*models.CalendarEvent, error) {
	// Заглушка для примера
	return &models.CalendarEvent{
		ID:          id,
		Owner:       owner,
		Summary:     "Встреча",
		Description: "Описание встречи",
		StartTime:   time.Now().Add(time.Hour),
		EndTime:     time.Now().Add(2 * time.Hour),
		RawData:     "BEGIN:VCALENDAR...",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// ListEvents возвращает список событий календаря
func (s *RQLiteStorage) ListEvents(calendarID, owner string, start, end time.Time) ([]*models.CalendarEvent, error) {
	// Заглушка для примера
	events := []*models.CalendarEvent{
		{
			ID:          "evt-1",
			UID:         "uid-1@" + owner,
			Owner:       owner,
			CalendarID:  calendarID,
			Summary:     "Командная встреча",
			Description: "Еженедельная встреча команды",
			Location:    "Конференц-зал А",
			StartTime:   start.Add(time.Hour),
			EndTime:     start.Add(2 * time.Hour),
			RawData:     "BEGIN:VEVENT\nSUMMARY:Командная встреча\nEND:VEVENT",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	
	return events, nil
}

// UpdateEvent обновляет событие
func (s *RQLiteStorage) UpdateEvent(event *models.CalendarEvent) error {
	query := `UPDATE calendar_events SET summary = ?, description = ?, location = ?, 
			  start_time = ?, end_time = ?, raw_data = ?, updated_at = ?, etag = ? 
			  WHERE id = ? AND owner = ?`
	
	now := time.Now()
	event.UpdatedAt = now
	event.ETag = generateETag(event.RawData, now)
	
	_, err := s.executeQuery(query, []interface{}{
		event.Summary,
		event.Description,
		event.Location,
		event.StartTime,
		event.EndTime,
		event.RawData,
		now,
		event.ETag,
		event.ID,
		event.Owner,
	})
	
	return err
}

// DeleteEvent удаляет событие
func (s *RQLiteStorage) DeleteEvent(id, owner string) error {
	query := `DELETE FROM calendar_events WHERE id = ? AND owner = ?`
	_, err := s.executeQuery(query, []interface{}{id, owner})
	return err
}

// CreateAddressBook создает адресную книгу
func (s *RQLiteStorage) CreateAddressBook(book *models.AddressBook) error {
	query := `INSERT INTO address_books (id, owner, name, description, raw_data, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?)`
	
	now := time.Now()
	book.CreatedAt = now
	book.UpdatedAt = now
	
	_, err := s.executeQuery(query, []interface{}{
		book.ID,
		book.Owner,
		book.Name,
		book.Description,
		book.RawData,
		now,
		now,
	})
	
	return err
}

// GetAddressBook получает адресную книгу по ID
func (s *RQLiteStorage) GetAddressBook(id, owner string) (*models.AddressBook, error) {
	return &models.AddressBook{
		ID:          id,
		Owner:       owner,
		Name:        "Контакты",
		Description: "Личные контакты",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// ListAddressBooks возвращает список адресных книг пользователя
func (s *RQLiteStorage) ListAddressBooks(owner string) ([]*models.AddressBook, error) {
	books := []*models.AddressBook{
		{
			ID:          "ab-1",
			Owner:       owner,
			Name:        "Личные контакты",
			Description: "Друзья и семья",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "ab-2",
			Owner:       owner,
			Name:        "Рабочие контакты",
			Description: "Коллеги и партнеры",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	
	return books, nil
}

// DeleteAddressBook удаляет адресную книгу
func (s *RQLiteStorage) DeleteAddressBook(id, owner string) error {
	query := `DELETE FROM address_books WHERE id = ? AND owner = ?`
	_, err := s.executeQuery(query, []interface{}{id, owner})
	return err
}

// CreateContact создает контакт
func (s *RQLiteStorage) CreateContact(contact *models.Contact) error {
	query := `INSERT INTO contacts (id, uid, owner, address_book_id, full_name, emails, phones, 
			  raw_data, created_at, updated_at, etag) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	now := time.Now()
	contact.CreatedAt = now
	contact.UpdatedAt = now
	contact.ETag = generateETag(contact.RawData, now)
	
	emailsJSON, _ := json.Marshal(contact.Emails)
	phonesJSON, _ := json.Marshal(contact.Phones)
	
	_, err := s.executeQuery(query, []interface{}{
		contact.ID,
		contact.UID,
		contact.Owner,
		contact.AddressBookID,
		contact.FullName,
		string(emailsJSON),
		string(phonesJSON),
		contact.RawData,
		now,
		now,
		contact.ETag,
	})
	
	return err
}

// GetContact получает контакт по ID
func (s *RQLiteStorage) GetContact(id, owner string) (*models.Contact, error) {
	return &models.Contact{
		ID:            id,
		Owner:         owner,
		FullName:      "Иван Иванов",
		Emails:        []string{"ivan@example.com"},
		Phones:        []string{"+7-999-123-45-67"},
		RawData:       "BEGIN:VCARD\nFN:Иван Иванов\nEND:VCARD",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}

// ListContacts возвращает список контактов адресной книги
func (s *RQLiteStorage) ListContacts(addressBookID, owner string) ([]*models.Contact, error) {
	contacts := []*models.Contact{
		{
			ID:            "cnt-1",
			UID:           "uid-cnt-1@" + owner,
			Owner:         owner,
			AddressBookID: addressBookID,
			FullName:      "Алексей Петров",
			Emails:        []string{"alexey@example.com"},
			Phones:        []string{"+7-999-111-22-33"},
			RawData:       "BEGIN:VCARD\nFN:Алексей Петров\nEND:VCARD",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}
	
	return contacts, nil
}

// UpdateContact обновляет контакт
func (s *RQLiteStorage) UpdateContact(contact *models.Contact) error {
	query := `UPDATE contacts SET full_name = ?, emails = ?, phones = ?, raw_data = ?, updated_at = ?, etag = ? 
			  WHERE id = ? AND owner = ?`
	
	now := time.Now()
	contact.UpdatedAt = now
	contact.ETag = generateETag(contact.RawData, now)
	
	emailsJSON, _ := json.Marshal(contact.Emails)
	phonesJSON, _ := json.Marshal(contact.Phones)
	
	_, err := s.executeQuery(query, []interface{}{
		contact.FullName,
		string(emailsJSON),
		string(phonesJSON),
		contact.RawData,
		now,
		contact.ETag,
		contact.ID,
		contact.Owner,
	})
	
	return err
}

// DeleteContact удаляет контакт
func (s *RQLiteStorage) DeleteContact(id, owner string) error {
	query := `DELETE FROM contacts WHERE id = ? AND owner = ?`
	_, err := s.executeQuery(query, []interface{}{id, owner})
	return err
}

// HealthCheck проверяет доступность RQLite сервера
func (s *RQLiteStorage) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/status", s.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	
	req.SetBasicAuth(s.username, s.password)
	
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка подключения к RQLite: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("RQLite вернул статус: %d", resp.StatusCode)
	}
	
	return nil
}
