// Package relay предоставляет функционал для работы с внешними почтовыми серверами
package relay

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"mail-matrix-server/internal/config"
	"mail-matrix-server/internal/storage"
)

// RelayManager управляет ретрансляцией писем на внешние серверы
type RelayManager struct {
	configs map[string]*config.ExternalServer
}

// NewRelayManager создает новый менеджер ретрансляции
func NewRelayManager(servers []config.ExternalServer) *RelayManager {
	configs := make(map[string]*config.ExternalServer)
	for i := range servers {
		configs[servers[i].Domain] = &servers[i]
	}

	return &RelayManager{
		configs: configs,
	}
}

// RelayEmail отправляет письмо на внешний сервер
func (r *RelayManager) RelayEmail(email *storage.Email) error {
	// Для каждого получателя определяем целевой сервер
	for _, recipient := range email.To {
		if err := r.sendToRecipient(email, recipient); err != nil {
			return fmt.Errorf("ошибка отправки получателю %s: %w", recipient, err)
		}
	}

	return nil
}

// sendToRecipient отправляет письмо конкретному получателю
func (r *RelayManager) sendToRecipient(email *storage.Email, recipient string) error {
	// Извлекаем домен получателя
	domain := extractDomain(recipient)
	if domain == "" {
		return fmt.Errorf("не удалось извлечь домен из адреса: %s", recipient)
	}

	// Проверяем есть ли конфигурация для этого домена
	serverConfig, exists := r.configs[domain]
	if !exists {
		// Пытаемся найти MX запись для домена
		return r.sendViaMX(domain, email, recipient)
	}

	// Отправляем через настроенный сервер
	return r.sendViaServer(serverConfig, email, recipient)
}

// sendViaMX отправляет письмо через MX запись домена
func (r *RelayManager) sendViaMX(domain string, email *storage.Email, recipient string) error {
	// Получаем MX записи для домена
	mxRecords, err := net.LookupMX(domain)
	if err != nil {
		return fmt.Errorf("не удалось получить MX записи для домена %s: %w", domain, err)
	}

	if len(mxRecords) == 0 {
		return fmt.Errorf("MX записи не найдены для домена %s", domain)
	}

	// Пробуем отправить через первый MX сервер
	mxHost := mxRecords[0].Host + ":25"

	return r.sendSMTP(mxHost, email, recipient, false, nil)
}

// sendViaServer отправляет письмо через настроенный сервер
func (r *RelayManager) sendViaServer(config *config.ExternalServer, email *storage.Email, recipient string) error {
	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)

	var auth smtp.Auth
	if config.Username != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.SMTPHost)
	}

	return r.sendSMTP(addr, email, recipient, config.UseTLS, auth)
}

// sendSMTP отправляет письмо через SMTP сервер
func (r *RelayManager) sendSMTP(addr string, email *storage.Email, recipient string, useTLS bool, auth smtp.Auth) error {
	// Формируем сообщение
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\n", email.From)
	fmt.Fprintf(&msg, "To: %s\r\n", recipient)
	fmt.Fprintf(&msg, "Subject: %s\r\n", email.Subject)
	fmt.Fprintf(&msg, "Date: %s\r\n", email.Date.Format(time.RFC1123Z))

	for key, value := range email.Headers {
		if key != "From" && key != "To" && key != "Subject" && key != "Date" {
			fmt.Fprintf(&msg, "%s: %s\r\n", key, value)
		}
	}

	fmt.Fprintf(&msg, "\r\n%s", email.Body)

	if useTLS {
		// Подключение с TLS
		tlsConfig := &tls.Config{
			ServerName: strings.Split(addr, ":")[0],
		}

		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("ошибка TLS подключения: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, strings.Split(addr, ":")[0])
		if err != nil {
			return err
		}
		defer client.Close()

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("ошибка аутентификации: %w", err)
			}
		}

		if err := client.Mail(email.From); err != nil {
			return err
		}

		if err := client.Rcpt(recipient); err != nil {
			return err
		}

		writer, err := client.Data()
		if err != nil {
			return err
		}

		_, err = writer.Write(msg.Bytes())
		if err != nil {
			return err
		}

		err = writer.Close()
		if err != nil {
			return err
		}

		return client.Quit()
	} else {
		// Обычное подключение
		var err error
		var client *smtp.Client

		if auth != nil {
			err = smtp.SendMail(addr, auth, email.From, []string{recipient}, msg.Bytes())
		} else {
			client, err = smtp.Dial(addr)
			if err != nil {
				return err
			}
			defer client.Close()

			if err := client.Mail(email.From); err != nil {
				return err
			}

			if err := client.Rcpt(recipient); err != nil {
				return err
			}

			writer, err := client.Data()
			if err != nil {
				return err
			}

			_, err = writer.Write(msg.Bytes())
			if err != nil {
				return err
			}

			err = writer.Close()
			if err != nil {
				return err
			}

			return client.Quit()
		}

		return err
	}
}

// FetchExternalEmails получает письма с внешних IMAP серверов
func (r *RelayManager) FetchExternalEmails(username string, storage *storage.FileStorage) error {
	// Находим конфигурацию для пользователя
	// В реальном проекте нужно сопоставить пользователя с конфигурацией внешнего сервера
	
	for _, config := range r.configs {
		if err := r.fetchFromServer(config, username, storage); err != nil {
			// Логируем ошибку но продолжаем с другими серверами
			continue
		}
	}

	return nil
}

// fetchFromServer получает письма с конкретного IMAP сервера
func (r *RelayManager) fetchFromServer(config *config.ExternalServer, username string, storage *storage.FileStorage) error {
	// Здесь должна быть реализация IMAP клиента для получения писем
	// В текущей версии используем упрощенный подход
	
	// Примечание: полная реализация IMAP клиента требует дополнительной библиотеки
	// и выходит за рамки базового примера
	
	return nil
}

// IsExternalDomain проверяет является ли домен внешним
func (r *RelayManager) IsExternalDomain(domain string) bool {
	_, exists := r.configs[domain]
	return exists
}

// extractDomain извлекает домен из email адреса
func extractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// ParseEmail парсит сырые данные письма
func ParseEmail(data []byte) (*storage.Email, error) {
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
