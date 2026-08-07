// Package smtprelay предоставляет функционал для отправки email через SMTP relay
package smtprelay

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"mail-matrix-server/internal/config"
)

// RelayClient - клиент для отправки email через SMTP relay
type RelayClient struct {
	host        string
	port        int
	username    string
	password    string
	fromAddress string
	fromName    string
	useTLS      bool
	useSTARTTLS bool
}

// EmailMessage представляет email сообщение для отправки
type EmailMessage struct {
	To          []string
	Subject     string
	Body        string
	HTMLBody    string // Опционально, для HTML писем
	FromName    string
	FromAddress string
	ReplyTo     string
	Headers     map[string]string
}

// NewRelayClient создает новый SMTP relay клиент
func NewRelayClient(cfg config.SMTPRelayConfig) *RelayClient {
	return &RelayClient{
		host:        cfg.Host,
		port:        cfg.Port,
		username:    cfg.Username,
		password:    cfg.Password,
		fromAddress: cfg.FromAddress,
		fromName:    cfg.FromName,
		useTLS:      cfg.UseTLS,
		useSTARTTLS: cfg.UseSTARTTLS,
	}
}

// SendEmail отправляет email сообщение
func (c *RelayClient) SendEmail(msg *EmailMessage) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("не указаны получатели")
	}
	if msg.Subject == "" {
		msg.Subject = "Без темы"
	}
	if msg.FromAddress == "" {
		msg.FromAddress = c.fromAddress
	}
	if msg.FromName == "" {
		msg.FromName = c.fromName
	}

	// Формируем тело письма в формате MIME
	body := c.buildMessage(msg)

	// Отправляем через SMTP
	return c.sendSMTP(msg.To, body)
}

// buildMessage формирует MIME сообщение
func (c *RelayClient) buildMessage(msg *EmailMessage) []byte {
	var buf bytes.Buffer

	// Заголовки
	from := mail.Address{Name: msg.FromName, Address: msg.FromAddress}
	fmt.Fprintf(&buf, "From: %s\r\n", from.String())

	toAddresses := make([]string, len(msg.To))
	for i, to := range msg.To {
		toAddresses[i] = to
	}
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(toAddresses, ", "))

	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")

	// Дополнительные заголовки
	if msg.ReplyTo != "" {
		fmt.Fprintf(&buf, "Reply-To: %s\r\n", msg.ReplyTo)
	}
	for key, value := range msg.Headers {
		fmt.Fprintf(&buf, "%s: %s\r\n", key, value)
	}

	// Content-Type зависит от наличия HTML версии
	if msg.HTMLBody != "" {
		// multipart/alternative для текста и HTML
		boundary := generateBoundary()
		fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
		fmt.Fprintf(&buf, "\r\n")

		// Текстовая версия
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: quoted-printable\r\n")
		fmt.Fprintf(&buf, "\r\n")
		fmt.Fprintf(&buf, "%s\r\n", msg.Body)

		// HTML версия
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: quoted-printable\r\n")
		fmt.Fprintf(&buf, "\r\n")
		fmt.Fprintf(&buf, "%s\r\n", msg.HTMLBody)

		// Конец multipart
		fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	} else {
		// Простое текстовое письмо
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: quoted-printable\r\n")
		fmt.Fprintf(&buf, "\r\n")
		fmt.Fprintf(&buf, "%s\r\n", msg.Body)
	}

	return buf.Bytes()
}

// sendSMTP отправляет сообщение через SMTP сервер
func (c *RelayClient) sendSMTP(to []string, message []byte) error {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)

	var auth smtp.Auth
	if c.username != "" && c.password != "" {
		auth = smtp.PlainAuth("", c.username, c.password, c.host)
	}

	if c.useTLS {
		// Прямое TLS соединение (SMTPS, порт 465)
		return c.sendWithTLS(addr, to, message, auth)
	} else if c.useSTARTTLS {
		// STARTTLS (порт 587)
		return c.sendWithSTARTTLS(addr, to, message, auth)
	} else {
		// Без шифрования (не рекомендуется для продакшена)
		return smtp.SendMail(addr, auth, c.fromAddress, to, message)
	}
}

// sendWithTLS отправляет через прямое TLS соединение
func (c *RelayClient) sendWithTLS(addr string, to []string, message []byte, auth smtp.Auth) error {
	tlsConfig := &tls.Config{
		ServerName: c.host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("ошибка TLS подключения: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.host)
	if err != nil {
		return fmt.Errorf("ошибка создания SMTP клиента: %w", err)
	}
	defer client.Close()

	return c.sendViaClient(client, auth, to, message)
}

// sendWithSTARTTLS отправляет через STARTTLS
func (c *RelayClient) sendWithSTARTTLS(addr string, to []string, message []byte, auth smtp.Auth) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("ошибка подключения к SMTP: %w", err)
	}
	defer client.Close()

	// Проверяем поддержку STARTTLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: c.host,
			MinVersion: tls.VersionTLS12,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("ошибка STARTTLS: %w", err)
		}
	}

	return c.sendViaClient(client, auth, to, message)
}

// sendViaClient выполняет отправку через готовый SMTP клиент
func (c *RelayClient) sendViaClient(client *smtp.Client, auth smtp.Auth, to []string, message []byte) error {
	// Аутентификация
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("ошибка аутентификации: %w", err)
		}
	}

	// Отправитель
	if err := client.Mail(c.fromAddress); err != nil {
		return fmt.Errorf("ошибка указания отправителя: %w", err)
	}

	// Получатели
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("ошибка указания получателя %s: %w", recipient, err)
		}
	}

	// Данные письма
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("ошибка начала передачи данных: %w", err)
	}

	_, err = writer.Write(message)
	if err != nil {
		return fmt.Errorf("ошибка записи данных: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("ошибка завершения передачи: %w", err)
	}

	return client.Quit()
}

// generateBoundary генерирует случайную границу для multipart
func generateBoundary() string {
	return fmt.Sprintf("----=_Part_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// CreateMatrixNotificationMessage создает email сообщение для уведомления о сообщении Matrix
func CreateMatrixNotificationMessage(senderID, senderName, roomName, messageText, userEmail string) *EmailMessage {
	subject := fmt.Sprintf("Новое сообщение в Matrix от %s", senderName)
	if roomName != "" {
		subject = fmt.Sprintf("Сообщение в комнате \"%s\" от %s", roomName, senderName)
	}

	body := fmt.Sprintf(
		`Пользователь %s (%s) отправил сообщение в Matrix:

%s

---
Это автоматическое уведомление от Matrix Mail Gateway`,
		senderName,
		senderID,
		messageText,
	)

	htmlBody := fmt.Sprintf(
		`<html><body>
<h2>Новое сообщение в Matrix</h2>
<p><strong>От:</strong> %s (%s)</p>
<p><strong>Комната:</strong> %s</p>
<hr>
<p>%s</p>
<hr>
<p style="color: gray; font-size: small;">Это автоматическое уведомление от Matrix Mail Gateway</p>
</body></html>`,
		senderName,
		senderID,
		roomName,
		messageText,
	)

	return &EmailMessage{
		To:       []string{userEmail},
		Subject:  subject,
		Body:     body,
		HTMLBody: htmlBody,
		Headers: map[string]string{
			"X-Matrix-Sender": senderID,
			"X-Matrix-Room":   roomName,
		},
	}
}

// ValidateConnection проверяет подключение к SMTP серверу
func (c *RelayClient) ValidateConnection() error {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)

	var err error
	var client *smtp.Client

	if c.useTLS {
		tlsConfig := &tls.Config{
			ServerName: c.host,
			MinVersion: tls.VersionTLS12,
		}
		conn, dialErr := tls.Dial("tcp", addr, tlsConfig)
		if dialErr != nil {
			return fmt.Errorf("ошибка TLS подключения: %w", dialErr)
		}
		client, err = smtp.NewClient(conn, c.host)
	} else {
		client, err = smtp.Dial(addr)
	}

	if err != nil {
		return fmt.Errorf("ошибка подключения к SMTP: %w", err)
	}
	defer client.Close()

	// Пробуем выполнить EHLO
	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("ошибка EHLO: %w", err)
	}

	return nil
}
