// Package config содержит конфигурацию сервера
package config

import (
	"encoding/json"
	"os"
)

// Config - основная структура конфигурации сервера
type Config struct {
	// SMTP порт для входящих соединений
	SMTPPort int `json:"smtp_port"`
	// IMAP порт для входящих соединений
	IMAPPort int `json:"imap_port"`
	// Домен сервера
	Domain string `json:"domain"`
	// Путь к хранилищу писем
	StoragePath string `json:"storage_path"`
	// Настройки для внешних почтовых серверов
	ExternalServers []ExternalServer `json:"external_servers"`
	// Настройки SMTP relay для отправки уведомлений
	SMTPRelay SMTPRelayConfig `json:"smtp_relay"`
	// Соответствия пользователей Matrix и Email
	MatrixUserMappings []MatrixUserMapping `json:"matrix_user_mappings"`
	// Настройки подключения к Matrix
	Matrix MatrixConfig `json:"matrix"`
}

// ExternalServer - конфигурация внешнего почтового сервера
type ExternalServer struct {
	// Домен, который обслуживает этот сервер
	Domain string `json:"domain"`
	// SMTP хост для отправки
	SMTPHost string `json:"smtp_host"`
	// SMTP порт
	SMTPPort int `json:"smtp_port"`
	// IMAP хост для получения
	IMAPHost string `json:"imap_host"`
	// IMAP порт
	IMAPPort int `json:"imap_port"`
	// Логин для аутентификации
	Username string `json:"username"`
	// Пароль для аутентификации
	Password string `json:"password"`
	// Использовать TLS
	UseTLS bool `json:"use_tls"`
}

// SMTPRelayConfig - настройки SMTP релея для отправки уведомлений
type SMTPRelayConfig struct {
	// Хост SMTP сервера (например, smtp.mail.ru)
	Host string `json:"host"`
	// Порт SMTP сервера (587 для STARTTLS, 465 для SMTPS)
	Port int `json:"port"`
	// Логин для аутентификации
	Username string `json:"username"`
	// Пароль для аутентификации
	Password string `json:"password"`
	// Адрес отправителя (должен совпадать с Username)
	FromAddress string `json:"from_address"`
	// Имя отправителя
	FromName string `json:"from_name"`
	// Использовать TLS (SMTPS)
	UseTLS bool `json:"use_tls"`
	// Использовать STARTTLS
	UseSTARTTLS bool `json:"use_starttls"`
}

// MatrixUserMapping - соответствие пользователя Matrix и Email
type MatrixUserMapping struct {
	// UserID в Matrix (например, @username:matrix.org)
	MatrixUserID string `json:"matrix_user_id"`
	// Email адрес для получения уведомлений
	Email string `json:"email"`
	// Список комнат Matrix для мониторинга (опционально, если пусто - все комнаты)
	RoomIDs []string `json:"room_ids,omitempty"`
	// Включить уведомления для этого пользователя
	Enabled bool `json:"enabled"`
}

// MatrixConfig - настройки подключения к Matrix
type MatrixConfig struct {
	// Homeserver URL (например, https://matrix.org)
	HomeserverURL string `json:"homeserver_url"`
	// Access токен для аутентификации бота
	AccessToken string `json:"access_token"`
	// UserID бота (например, @botname:matrix.org)
	UserID string `json:"user_id"`
	// Включить синхронизацию
	Enabled bool `json:"enabled"`
	// Таймаут для запросов к Matrix API (в секундах)
	RequestTimeout int `json:"request_timeout"`
	// Интервал повторной синхронизации при ошибке (в секундах)
	ReconnectDelay int `json:"reconnect_delay"`
}

// LoadConfig загружает конфигурацию из JSON файла
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() *Config {
	return &Config{
		SMTPPort:    2525,
		IMAPPort:    143,
		Domain:      "localhost",
		StoragePath: "./maildata",
		Matrix: MatrixConfig{
			RequestTimeout: 30,
			ReconnectDelay: 10,
		},
	}
}
