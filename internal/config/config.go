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
	}
}
