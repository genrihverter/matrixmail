// Основной файл сервера Matrix Mail с поддержкой уведомлений на Email
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
	"mail-matrix-server/internal/notifier"
	"mail-matrix-server/internal/relay"
	"mail-matrix-server/internal/storage"

	"github.com/emersion/go-imap/server"
	goSmtp "github.com/emersion/go-smtp"
)

func main() {
	// Парсим флаги командной строки
	configPath := flag.String("config", "", "Путь к файлу конфигурации")
	flag.Parse()

	// Инициализируем логгер
	initLogger()

	// Загружаем конфигурацию
	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.LoadConfig(*configPath)
		if err != nil {
			log.Fatalf("Ошибка загрузки конфигурации: %v", err)
		}
		log.Printf("Загружена конфигурация из файла: %s", *configPath)
	} else {
		cfg = config.DefaultConfig()
		log.Println("Используется конфигурация по умолчанию")
	}

	// Создаем хранилище
	storageInstance, err := storage.NewFileStorage(cfg.StoragePath)
	if err != nil {
		log.Fatalf("Ошибка создания хранилища: %v", err)
	}

	// Создаем менеджер ретрансляции для обычной почты
	relayManager := relay.NewRelayManager(cfg.ExternalServers)

	// Создаем SMTP бэкенд
	smtpBackend := handlers.NewSMTPBackend(storageInstance)

	// Создаем SMTP сервер
	smtpServer := goSmtp.NewServer(smtpBackend)
	smtpServer.Addr = fmt.Sprintf(":%d", cfg.SMTPPort)
	smtpServer.Domain = cfg.Domain
	smtpServer.AllowInsecureAuth = true // Для тестирования, в продакшене использовать TLS

	// Создаем IMAP бэкенд
	imapBackend := handlers.NewIMAPBackend(storageInstance)

	// Создаем IMAP сервер
	imapServer := server.New(imapBackend)
	imapServer.Addr = fmt.Sprintf(":%d", cfg.IMAPPort)

	// Создаем и запускаем нотификатор Matrix -> Email
	var matrixNotifier *notifier.Notifier
	if cfg.Matrix.Enabled && cfg.SMTPRelay.Host != "" {
		log.Println("Инициализация нотификатора Matrix -> Email...")
		
		matrixNotifier, err = notifier.NewNotifier(
			cfg.Matrix,
			cfg.SMTPRelay,
			cfg.MatrixUserMappings,
		)
		if err != nil {
			log.Printf("Предупреждение: не удалось создать нотификатор: %v", err)
			log.Println("Сервер будет работать без уведомлений Matrix -> Email")
		} else {
			// Проверяем подключение к SMTP
			if err := matrixNotifier.ValidateSMTPConnection(); err != nil {
				log.Printf("Предупреждение: не удалось подключиться к SMTP relay: %v", err)
				log.Println("Проверьте настройки SMTP в конфигурации")
			} else {
				log.Println("Подключение к SMTP relay успешно")
			}

			// Запускаем нотификатор
			if err := matrixNotifier.Start(); err != nil {
				log.Printf("Предупреждение: не удалось запустить нотификатор: %v", err)
			} else {
				log.Println("Нотификатор Matrix -> Email запущен")
			}
		}
	} else {
		log.Println("Нотификатор Matrix -> Email отключен в конфигурации")
	}

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

		// Останавливаем нотификатор
		if matrixNotifier != nil {
			if err := matrixNotifier.Stop(); err != nil {
				log.Printf("Ошибка остановки нотификатора: %v", err)
			} else {
				log.Println("Нотификатор остановлен")
			}
		}

		if err := smtpServer.Shutdown(ctx); err != nil {
			log.Printf("Ошибка остановки SMTP сервера: %v", err)
		}

		if err := imapServer.Close(); err != nil {
			log.Printf("Ошибка остановки IMAP сервера: %v", err)
		}

		log.Println("Сервер успешно остановлен")
	}

	// Используем relayManager чтобы избежать предупреждения о неиспользуемой переменной
	_ = relayManager
}

// initLogger инициализирует логгер
func initLogger() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("[Mail-Matrix-Server] ")
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
