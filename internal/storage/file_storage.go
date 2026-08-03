// Package storage предоставляет хранилище для электронных писем
package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Email представляет электронное письмо
type Email struct {
	// ID письма
	ID string `json:"id"`
	// От кого
	From string `json:"from"`
	// Кому
	To []string `json:"to"`
	// Тема
	Subject string `json:"subject"`
	// Дата
	Date time.Time `json:"date"`
	// Тело письма
	Body string `json:"body"`
	// Заголовки
	Headers map[string]string `json:"headers"`
	// Путь к файлу
	FilePath string `json:"file_path"`
}

// Mailbox представляет почтовый ящик пользователя
type Mailbox struct {
	// Имя пользователя
	Username string `json:"username"`
	// Письма
	Emails []*Email `json:"emails"`
	// Мьютекс для потокобезопасности
	mu sync.RWMutex
}

// FileStorage - файловое хранилище писем
type FileStorage struct {
	// Базовый путь для хранения
	basePath string
	// Мапа почтовых ящиков
	mailboxes map[string]*Mailbox
	// Мьютекс для потокобезопасности
	mu sync.RWMutex
}

// NewFileStorage создает новое файловое хранилище
func NewFileStorage(basePath string) (*FileStorage, error) {
	// Создаем директорию если не существует
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию хранилища: %w", err)
	}

	storage := &FileStorage{
		basePath:  basePath,
		mailboxes: make(map[string]*Mailbox),
	}

	// Загружаем существующие почтовые ящики
	if err := storage.loadMailboxes(); err != nil {
		return nil, err
	}

	return storage, nil
}

// loadMailboxes загружает существующие почтовые ящики с диска
func (s *FileStorage) loadMailboxes() error {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		username := entry.Name()
		mailboxPath := filepath.Join(s.basePath, username)

		// Загружаем метаданные почтового ящика
		mailboxFile := filepath.Join(mailboxPath, "mailbox.json")
		data, err := os.ReadFile(mailboxFile)
		if err != nil {
			// Если файла нет, создаем новый почтовый ящик
			mailbox := &Mailbox{
				Username: username,
				Emails:   make([]*Email, 0),
			}
			s.mailboxes[username] = mailbox
			continue
		}

		var mailbox Mailbox
		if err := json.Unmarshal(data, &mailbox); err != nil {
			return fmt.Errorf("ошибка загрузки почтового ящика %s: %w", username, err)
		}

		s.mailboxes[username] = &mailbox
	}

	return nil
}

// CreateMailbox создает новый почтовый ящик
func (s *FileStorage) CreateMailbox(username string) (*Mailbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.mailboxes[username]; exists {
		return nil, fmt.Errorf("почтовый ящик уже существует")
	}

	mailboxPath := filepath.Join(s.basePath, username)
	if err := os.MkdirAll(mailboxPath, 0755); err != nil {
		return nil, err
	}

	mailbox := &Mailbox{
		Username: username,
		Emails:   make([]*Email, 0),
	}

	s.mailboxes[username] = mailbox

	// Сохраняем метаданные
	if err := s.saveMailbox(mailbox); err != nil {
		return nil, err
	}

	return mailbox, nil
}

// GetMailbox получает почтовый ящик по имени пользователя
func (s *FileStorage) GetMailbox(username string) (*Mailbox, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mailbox, exists := s.mailboxes[username]
	return mailbox, exists
}

// SaveEmail сохраняет письмо в почтовый ящик
func (s *FileStorage) SaveEmail(username string, email *Email) error {
	mailbox, exists := s.GetMailbox(username)
	if !exists {
		// Создаем почтовый ящик если не существует
		var err error
		mailbox, err = s.CreateMailbox(username)
		if err != nil {
			return err
		}
	}

	mailbox.mu.Lock()
	defer mailbox.mu.Unlock()

	// Генерируем уникальный ID и путь к файлу
	email.ID = generateID()
	email.FilePath = filepath.Join(s.basePath, username, email.ID+".eml")

	// Сохраняем письмо в файл
	if err := s.saveEmailToFile(email); err != nil {
		return err
	}

	// Добавляем в список писем
	mailbox.Emails = append(mailbox.Emails, email)

	// Сохраняем метаданные почтового ящика
	return s.saveMailbox(mailbox)
}

// saveEmailToFile сохраняет письмо в файл
func (s *FileStorage) saveEmailToFile(email *Email) error {
	file, err := os.Create(email.FilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Формируем содержимое письма в формате RFC 822
	fmt.Fprintf(file, "From: %s\r\n", email.From)
	fmt.Fprintf(file, "To: %s\r\n", joinStrings(email.To))
	fmt.Fprintf(file, "Subject: %s\r\n", email.Subject)
	fmt.Fprintf(file, "Date: %s\r\n", email.Date.Format(time.RFC1123Z))
	
	for key, value := range email.Headers {
		fmt.Fprintf(file, "%s: %s\r\n", key, value)
	}
	
	fmt.Fprintf(file, "\r\n%s", email.Body)

	return nil
}

// saveMailbox сохраняет метаданные почтового ящика
func (s *FileStorage) saveMailbox(mailbox *Mailbox) error {
	mailboxPath := filepath.Join(s.basePath, mailbox.Username)
	mailboxFile := filepath.Join(mailboxPath, "mailbox.json")

	data, err := json.MarshalIndent(mailbox, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(mailboxFile, data, 0644)
}

// GetEmail получает письмо по ID
func (s *FileStorage) GetEmail(username, emailID string) (*Email, error) {
	mailbox, exists := s.GetMailbox(username)
	if !exists {
		return nil, fmt.Errorf("почтовый ящик не найден")
	}

	mailbox.mu.RLock()
	defer mailbox.mu.RUnlock()

	for _, email := range mailbox.Emails {
		if email.ID == emailID {
			return email, nil
		}
	}

	return nil, fmt.Errorf("письмо не найдено")
}

// DeleteEmail удаляет письмо из почтового ящика
func (s *FileStorage) DeleteEmail(username, emailID string) error {
	mailbox, exists := s.GetMailbox(username)
	if !exists {
		return fmt.Errorf("почтовый ящик не найден")
	}

	mailbox.mu.Lock()
	defer mailbox.mu.Unlock()

	// Находим и удаляем письмо
	for i, email := range mailbox.Emails {
		if email.ID == emailID {
			// Удаляем файл
			if err := os.Remove(email.FilePath); err != nil && !os.IsNotExist(err) {
				return err
			}

			// Удаляем из списка
			mailbox.Emails = append(mailbox.Emails[:i], mailbox.Emails[i+1:]...)
			
			// Сохраняем метаданные
			return s.saveMailbox(mailbox)
		}
	}

	return fmt.Errorf("письмо не найдено")
}

// ListEmails возвращает список писем в почтовом ящике
func (s *FileStorage) ListEmails(username string) ([]*Email, error) {
	mailbox, exists := s.GetMailbox(username)
	if !exists {
		return nil, fmt.Errorf("почтовый ящик не найден")
	}

	mailbox.mu.RLock()
	defer mailbox.mu.RUnlock()

	// Возвращаем копию списка
	emails := make([]*Email, len(mailbox.Emails))
	copy(emails, mailbox.Emails)
	return emails, nil
}

// ReadEmailContent читает содержимое письма из файла
func (s *FileStorage) ReadEmailContent(email *Email) (io.ReadCloser, error) {
	return os.Open(email.FilePath)
}

// generateID генерирует уникальный ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// joinStrings объединяет строки через запятую
func joinStrings(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += ", " + strs[i]
	}
	return result
}
