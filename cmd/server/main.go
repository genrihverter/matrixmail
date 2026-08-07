// Основной файл сервера уведомлений Matrix -> Email
package main

import (
"flag"
"log"
"os"
"os/signal"
"syscall"

"mail-matrix-server/internal/config"
"mail-matrix-server/internal/notifier"
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

// Обработка сигналов завершения
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

// Ожидаем сигнал завершения
<-sigChan
log.Println("Получен сигнал завершения, останавливаем работу...")

// Останавливаем нотификатор
if matrixNotifier != nil {
if err := matrixNotifier.Stop(); err != nil {
log.Printf("Ошибка остановки нотификатора: %v", err)
} else {
log.Println("Нотификатор остановлен")
}
}

log.Println("Сервер успешно остановлен")
}

// initLogger инициализирует логгер
func initLogger() {
log.SetFlags(log.LstdFlags | log.Lshortfile)
log.SetPrefix("[Mail-Matrix-Server] ")
}
