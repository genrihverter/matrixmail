// Package handlers содержит обработчики для SMTP и IMAP протоколов
package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"mail-matrix-server/internal/storage"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-smtp"
)

// SMTPBackend реализует бэкенд для SMTP сервера
type SMTPBackend struct {
	storage *storage.FileStorage
}

// NewSMTPBackend создает новый SMTP бэкенд
func NewSMTPBackend(s *storage.FileStorage) *SMTPBackend {
	return &SMTPBackend{
		storage: s,
	}
}

// NewSession создает новую SMTP сессию
func (b *SMTPBackend) NewSession(conn *smtp.Conn) (smtp.Session, error) {
	return &SMTPSession{
		backend: b,
		conn:    conn,
	}, nil
}

// SMTPSession представляет SMTP сессию
type SMTPSession struct {
	backend *SMTPBackend
	conn    *smtp.Conn
	from    string
	to      []string
}

// Mail обрабатывает команду MAIL FROM
func (s *SMTPSession) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

// Rcpt обрабатывает команду RCPT TO
func (s *SMTPSession) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

// Data обрабатывает данные письма
func (s *SMTPSession) Data(r io.Reader) error {
	// Читаем все данные письма
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// Парсим письмо
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return err
	}

	// Читаем тело письма
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return err
	}

	// Создаем структуру Email
	email := &storage.Email{
		From:    s.from,
		To:      s.to,
		Subject: msg.Header.Get("Subject"),
		Date:    time.Now(),
		Body:    string(body),
		Headers: make(map[string]string),
	}

	// Копируем заголовки
	for key, values := range msg.Header {
		if len(values) > 0 {
			email.Headers[key] = strings.Join(values, ", ")
		}
	}

	// Сохраняем письмо для каждого получателя
	for _, recipient := range s.to {
		// Извлекаем имя пользователя из email адреса
		username := extractUsername(recipient)
		
		if err := s.backend.storage.SaveEmail(username, email); err != nil {
			return fmt.Errorf("ошибка сохранения письма: %w", err)
		}
	}

	return nil
}

// Reset сбрасывает сессию
func (s *SMTPSession) Reset() {
	s.from = ""
	s.to = nil
}

// Logout завершает сессию
func (s *SMTPSession) Logout() error {
	return nil
}

// IMAPBackend реализует бэкенд для IMAP сервера
type IMAPBackend struct {
	storage *storage.FileStorage
}

// NewIMAPBackend создает новый IMAP бэкенд
func NewIMAPBackend(s *storage.FileStorage) *IMAPBackend {
	return &IMAPBackend{
		storage: s,
	}
}

// Login аутентифицирует пользователя
func (b *IMAPBackend) Login(username, password string) (backend.User, error) {
	// Проверяем существование почтового ящика
	mailbox, exists := b.storage.GetMailbox(username)
	if !exists {
		// Создаем новый почтовый ящик
		var err error
		mailbox, err = b.storage.CreateMailbox(username)
		if err != nil {
			return nil, err
		}
	}

	return &IMAPUser{
		username: username,
		password: password,
		backend:  b,
		mailbox:  mailbox,
	}, nil
}

// IMAPUser представляет пользователя IMAP
type IMAPUser struct {
	username string
	password string
	backend  *IMAPBackend
	mailbox  *storage.Mailbox
}

// Username возвращает имя пользователя
func (u *IMAPUser) Username() string {
	return u.username
}

// ListMailboxes возвращает список почтовых ящиков
func (u *IMAPUser) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	// Возвращаем только INBOX
	return []backend.Mailbox{&IMAPMailbox{user: u, name: "INBOX"}}, nil
}

// GetMailbox получает почтовый ящик по имени
func (u *IMAPUser) GetMailbox(name string) (backend.Mailbox, error) {
	if name != "INBOX" {
		return nil, backend.ErrNoSuchMailbox
	}
	return &IMAPMailbox{user: u, name: "INBOX"}, nil
}

// CreateMailbox создает новый почтовый ящик
func (u *IMAPUser) CreateMailbox(name string) error {
	// В нашей реализации один почтовый ящик на пользователя
	if name == "INBOX" {
		return fmt.Errorf("почтовый ящик уже существует")
	}
	return fmt.Errorf("создание произвольных ящиков не поддерживается")
}

// DeleteMailbox удаляет почтовый ящик
func (u *IMAPUser) DeleteMailbox(name string) error {
	return fmt.Errorf("удаление почтовых ящиков не поддерживается")
}

// RenameMailbox переименовывает почтовый ящик
func (u *IMAPUser) RenameMailbox(oldName, newName string) error {
	return fmt.Errorf("переименование почтовых ящиков не поддерживается")
}

// Logout завершает сессию
func (u *IMAPUser) Logout() error {
	return nil
}

// IMAPMailbox представляет почтовый ящик IMAP
type IMAPMailbox struct {
	user *IMAPUser
	name string
}

// Name возвращает имя почтового ящика
func (m *IMAPMailbox) Name() string {
	return m.name
}

// Info возвращает информацию о почтовом ящике
func (m *IMAPMailbox) Info() (*imap.MailboxInfo, error) {
	return &imap.MailboxInfo{
		Name: m.name,
	}, nil
}

// Status возвращает статус почтового ящика
func (m *IMAPMailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	status := &imap.MailboxStatus{
		Name: m.name,
	}

	for _, item := range items {
		switch item {
		case imap.StatusMessages:
			count, err := m.Messages()
			if err != nil {
				return nil, err
			}
			status.Messages = count
		case imap.StatusRecent:
			count, err := m.Recent()
			if err != nil {
				return nil, err
			}
			status.Recent = count
		case imap.StatusUnseen:
			count, err := m.Unseen()
			if err != nil {
				return nil, err
			}
			status.Unseen = count
		case imap.StatusUidNext:
			uid, err := m.UIDNext()
			if err != nil {
				return nil, err
			}
			status.UidNext = uid
		case imap.StatusUidValidity:
			uid, err := m.UIDValidity()
			if err != nil {
				return nil, err
			}
			status.UidValidity = uid
		}
	}

	return status, nil
}

// SetSubscribed устанавливает подписку на почтовый ящик
func (m *IMAPMailbox) SetSubscribed(subscribed bool) error {
	return nil
}

// Check выполняет проверку почтового ящика
func (m *IMAPMailbox) Check() error {
	return nil
}

// ListMessages получает сообщения
func (m *IMAPMailbox) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	defer close(ch)

	emails, err := m.user.backend.storage.ListEmails(m.user.username)
	if err != nil {
		return err
	}

	for i, email := range emails {
		seqNum := uint32(i + 1)
		// Проверяем соответствие seqSet
		if !seqset.Contains(seqNum) {
			continue
		}

		msg := &imap.Message{
			SeqNum: seqNum,
			Uid:    seqNum,
			Items:  make(map[imap.FetchItem]interface{}),
		}

		for _, item := range items {
			switch item {
			case imap.FetchEnvelope:
				msg.Items[item] = createEnvelope(email)
			case imap.FetchFlags:
				msg.Items[item] = []string{}
			case imap.FetchInternalDate:
				msg.Items[item] = email.Date
			case imap.FetchRFC822:
				reader, err := m.user.backend.storage.ReadEmailContent(email)
				if err != nil {
					return err
				}
				data, err := io.ReadAll(reader)
				if err != nil {
					return err
				}
				msg.Items[item] = data
			case imap.FetchRFC822Header:
				reader, err := m.user.backend.storage.ReadEmailContent(email)
				if err != nil {
					return err
				}
				data, err := io.ReadAll(reader)
				if err != nil {
					return err
				}
				// Извлекаем только заголовки
				msg.Items[item] = extractHeaders(data)
			case imap.FetchRFC822Text:
				msg.Items[item] = []byte(email.Body)
			case imap.FetchUid:
				msg.Items[item] = msg.Uid
			case imap.FetchBodyStructure:
				msg.Items[item] = createBodyStructure(email)
			}
		}

		ch <- msg
	}

	return nil
}

// SearchMessages ищет сообщения
func (m *IMAPMailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	emails, err := m.user.backend.storage.ListEmails(m.user.username)
	if err != nil {
		return nil, err
	}

	var results []uint32
	for i := range emails {
		results = append(results, uint32(i+1))
	}

	return results, nil
}

// CreateMessage добавляет новое сообщение
func (m *IMAPMailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	// Читаем данные
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	// Парсим письмо
	email, err := parseEmailFromData(data)
	if err != nil {
		return err
	}

	// Сохраняем письмо
	return m.user.backend.storage.SaveEmail(m.user.username, email)
}

// UpdateMessagesFlags обновляет флаги сообщений
func (m *IMAPMailbox) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, flags *imap.FlagsUpdate, ch chan<- *imap.Message) error {
	defer close(ch)
	return fmt.Errorf("изменение флагов не поддерживается")
}

// Expunge удаляет помеченные на удаление сообщения
func (m *IMAPMailbox) Expunge(seqSet *imap.SeqSet) (uint32, error) {
	return 0, fmt.Errorf("удаление сообщений не поддерживается")
}

// parseEmailFromData парсит email из сырых данных
func parseEmailFromData(data []byte) (*storage.Email, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return nil, err
	}

	email := &storage.Email{
		From:    msg.Header.Get("From"),
		Subject: msg.Header.Get("Subject"),
		Date:    time.Now(),
		Body:    string(body),
		Headers: make(map[string]string),
	}

	// Парсим получателей
	if to := msg.Header.Get("To"); to != "" {
		addresses, err := mail.ParseAddressList(to)
		if err == nil {
			for _, addr := range addresses {
				email.To = append(email.To, addr.Address)
			}
		}
	}

	// Копируем заголовки
	for key, values := range msg.Header {
		if len(values) > 0 {
			email.Headers[key] = strings.Join(values, ", ")
		}
	}

	return email, nil
}

// createEnvelope создает конверт сообщения IMAP
func createEnvelope(email *storage.Email) *imap.Envelope {
	return &imap.Envelope{
		Date:    email.Date,
		Subject: email.Subject,
		From:    createAddressList([]string{email.From}),
		To:      createAddressList(email.To),
	}
}

// createAddressList создает список адресов
func createAddressList(addresses []string) []*imap.Address {
	result := make([]*imap.Address, 0, len(addresses))
	for _, addr := range addresses {
		if parsed, err := mail.ParseAddress(addr); err == nil {
			// Разбираем адрес на части
			atIndex := strings.LastIndex(parsed.Address, "@")
			mailbox := parsed.Address
			host := ""
			if atIndex != -1 {
				mailbox = parsed.Address[:atIndex]
				host = parsed.Address[atIndex+1:]
			}
			result = append(result, &imap.Address{
				PersonalName: parsed.Name,
				MailboxName:  mailbox,
				HostName:     host,
			})
		} else {
			// Если не удалось распарсить, используем как есть
			atIndex := strings.LastIndex(addr, "@")
			mailbox := addr
			host := ""
			if atIndex != -1 {
				mailbox = addr[:atIndex]
				host = addr[atIndex+1:]
			}
			result = append(result, &imap.Address{
				MailboxName: mailbox,
				HostName:    host,
			})
		}
	}
	return result
}

// createBodyStructure создает структуру тела сообщения
func createBodyStructure(email *storage.Email) *imap.BodyStructure {
	return &imap.BodyStructure{
		MIMEType:    "text",
		MIMESubType: "plain",
		Size:        uint32(len(email.Body)),
	}
}

// extractHeaders извлекает заголовки из сырых данных
func extractHeaders(data []byte) []byte {
	// Находим разделитель между заголовками и телом
	idx := bytes.Index(data, []byte("\r\n\r\n"))
	if idx == -1 {
		return data
	}
	return data[:idx]
}

// extractUsername извлекает имя пользователя из email адреса
func extractUsername(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) > 0 {
		return parts[0]
	}
	return email
}
