// Основной файл сервера Matrix Mail
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mail-matrix-server/internal/config"
	"mail-matrix-server/internal/handlers"
	"mail-matrix-server/internal/relay"
	"mail-matrix-server/internal/storage"

	"github.com/emersion/go-imap/server"
	goSmtp "github.com/emersion/go-smtp"
)

func main() {
	// Парсим флаги командной строки
	configPath := flag.String("config", "", "Путь к файлу конфигурации")
	flag.Parse()

	// Загружаем конфигурацию
	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.LoadConfig(*configPath)
		if err != nil {
			log.Fatalf("Ошибка загрузки конфигурации: %v", err)
		}
	} else {
		cfg = config.DefaultConfig()
		log.Println("Используется конфигурация по умолчанию")
	}

	// Создаем хранилище
	storage, err := storage.NewFileStorage(cfg.StoragePath)
	if err != nil {
		log.Fatalf("Ошибка создания хранилища: %v", err)
	}

	// Создаем менеджер ретрансляции
	relayManager := relay.NewRelayManager(cfg.ExternalServers)

	// Создаем SMTP бэкенд
	smtpBackend := handlers.NewSMTPBackend(storage)

	// Создаем SMTP сервер
	smtpServer := goSmtp.NewServer(smtpBackend)
	smtpServer.Addr = fmt.Sprintf(":%d", cfg.SMTPPort)
	smtpServer.Domain = cfg.Domain
	smtpServer.AllowInsecureAuth = true // Для тестирования, в продакшене использовать TLS

	// Создаем IMAP бэкенд
	imapBackend := handlers.NewIMAPBackend(storage)

	// Создаем IMAP сервер
	imapServer := server.New(imapBackend)
	imapServer.Addr = fmt.Sprintf(":%d", cfg.IMAPPort)

	// Каналы для управления серверами
	smtpErr := make(chan error, 1)
	imapErr := make(chan error, 1)

	// Запускаем SMTP сервер
	go func() {
		log.Printf("SMTP сервер запущен на порту %d", cfg.SMTPPort)
		if err := smtpServer.ListenAndServe(); err != nil {
			smtpErr <- err
		}
	}()

	// Запускаем IMAP сервер
	go func() {
		log.Printf("IMAP сервер запущен на порту %d", cfg.IMAPPort)
		if err := imapServer.Serve(); err != nil {
			imapErr <- err
		}
	}()

	// Обработка сигналов завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Ожидаем завершение или сигнал
	select {
	case err := <-smtpErr:
		log.Fatalf("Ошибка SMTP сервера: %v", err)
	case err := <-imapErr:
		log.Fatalf("Ошибка IMAP сервера: %v", err)
	case sig := <-sigChan:
		log.Printf("Получен сигнал %v, завершаем работу...", sig)
		
		// Graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := smtpServer.Shutdown(ctx); err != nil {
			log.Printf("Ошибка остановки SMTP сервера: %v", err)
		}

		if err := imapServer.Close(); err != nil {
			log.Printf("Ошибка остановки IMAP сервера: %v", err)
		}

		log.Println("Сервер успешно остановлен")
	}
}

// initLogger инициализирует логгер
func initLogger() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix "[Mail-Matrix-Server] "
}

// checkPort проверяет доступен ли порт
func checkPort(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}
