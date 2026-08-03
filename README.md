# Matrix Mail & PIM Server

Полноценный почтовый и PIM (Personal Information Management) сервер на Go, поддерживающий протоколы **SMTP**, **IMAP**, **CalDAV** и **CardDAV**. Сервер предназначен для интеграции с экосистемой Matrix, обеспечивая работу электронной почты, календарей и контактов с возможностью синхронизации между различными клиентами.

В качестве распределенного хранилища данных используется **RQLite** (SQLite поверх Raft), что обеспечивает отказоустойчивость и возможность масштабирования кластера.

## Особенности

*   **SMTP Сервер**: Прием входящей почты, поддержка аутентификации, ретрансляция (Relay) на внешние серверы через MX-записи.
*   **IMAP Сервер**: Доступ к почтовым ящикам, поддержка папок, флагов и поиска сообщений.
*   **CalDAV Сервер**: Управление календарями (RFC 4791), поддержка событий (VEVENT), задач (VTODO).
*   **CardDAV Сервер**: Управление контактами (RFC 6352), поддержка vCard.
*   **Хранилище RQLite**: Распределенное, консистентное хранилище для метаданных писем, календарей и контактов.
*   **Безопасность**: Поддержка Basic Auth, защита от анонимного релея.
*   **Производительность**: Минималистичная архитектура, отсутствие тяжелых ORM, прямая работа с SQL и буферами.
*   **Тестируемость**: Покрытие ключевых модулей юнит-тестами.

## Архитектура

Проект структурирован по принципу разделения ответственности:

```text
/cmd
  /server         # Точка входа приложения
/internal
  /config         # Конфигурация приложения
  /store          # Слой работы с RQLite (SQL запросы, миграции)
  /smtp           # Реализация SMTP сервера (backend для emersion/go-smtp)
  /imap           # Реализация IMAP сервера (backend для emersion/go-imap)
  /caldav         # Обработчики CalDAV (WebDAV + XML)
  /carddav        # Обработчики CardDAV (WebDAV + XML)
  /models         # Структуры данных (User, Message, Event, Contact)
  /auth           # Логика аутентификации
/pkg
  /rqlite-client  # Утилиты для подключения к RQLite
/tests            # Интеграционные тесты
```

## Требования

*   **Go**: версия 1.21 или выше.
*   **RQLite**: запущенный кластер или одиночный узел (можно использовать Docker).
*   **Docker** (опционально): Для быстрого развертывания зависимостей.

## Быстрый старт

### 1. Запуск RQLite

Самый простой способ поднять хранилище — использовать Docker:

```bash
docker run -d --name rqlite \
  -p 4000:4000 -p 4001:4001 \
  rqlite/rqlite:latest \
  -node-id 1 -http-addr 0.0.0.0:4000 -raft-addr 0.0.0.0:4001 /rqlite
```

Проверка доступности:
```bash
curl http://localhost:4000/status
```

### 2. Конфигурация

Создайте файл `config.yaml` в корне проекта:

```yaml
server:
  smtp_port: 2525
  imap_port: 143
  dav_port: 8080
  domain: "example.com"

rqlite:
  address: "http://localhost:4000"
  # Для кластера можно указать несколько адресов
  
auth:
  # В реальной системе используйте интеграцию с Matrix Auth API
  # Здесь для примера статические пользователи
  users:
    - username: "alice"
      password: "secret123"
    - username: "bob"
      password: "password456"

storage:
  # Путь для хранения больших вложений (опционально, если не в RQLite)
  attachments_path: "./data/attachments"
```

### 3. Установка зависимостей и запуск

```bash
# Инициализация модуля (если еще не сделана)
go mod init matrix-mail-server

# Установка зависимостей
go mod tidy

# Запуск сервера
go run ./cmd/server -config config.yaml
```

Сервер запустится и автоматически выполнит миграции схемы БД в RQLite при первом старте.

## Использование

### Почта (SMTP/IMAP)

**Отправка письма (SMTP):**
```bash
echo "Тестовое письмо" | sendmail -S localhost:2525 -f alice@example.com bob@example.com
```
Или через telnet:
```bash
telnet localhost 2525
HELO client
MAIL FROM:<alice@example.com>
RCPT TO:<bob@example.com>
DATA
Subject: Привет
Это тест.
.
QUIT
```

**Чтение почты (IMAP):**
Подключитесь любым почтовым клиентом (Thunderbird, Outlook, Apple Mail):
*   **Сервер**: `localhost`
*   **Порт IMAP**: `143`
*   **Логин/Пароль**: из конфига (например, `alice` / `secret123`)
*   **SSL/TLS**: Отключено (для локального теста), в продакшене используйте StartTLS.

### Календари и Контакты (CalDAV/CardDAV)

Сервер доступен по адресу `http://localhost:8080`.

**Базовые URL:**
*   Календари: `http://localhost:8080/dav/calendars/{username}/`
*   Контакты: `http://localhost:8080/dav/contacts/{username}/`

**Пример создания события (cURL):**

```bash
curl -X PUT http://localhost:8080/dav/calendars/alice/event1.ics \
  -u alice:secret123 \
  -H "Content-Type: text/calendar" \
  -H "If-None-Match: *" \
  --data-binary @- << EOF
BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Matrix Server//EN
BEGIN:VEVENT
UID:event1@example.com
DTSTAMP:20231027T100000Z
DTSTART:20231028T090000Z
DTEND:20231028T100000Z
SUMMARY:Встреча с командой
END:VEVENT
END:VCALENDAR
EOF
```

**Пример добавления контакта:**

```bash
curl -X PUT http://localhost:8080/dav/contacts/alice/contact1.vcf \
  -u alice:secret123 \
  -H "Content-Type: text/vcard" \
  --data-binary @- << EOF
BEGIN:VCARD
VERSION:3.0
FN:Иван Иванов
N:Иванов;Иван;;;
EMAIL;TYPE=WORK:ivan@example.com
TEL;TYPE=CELL:+79990000000
END:VCARD
EOF
```

### Интеграция с клиентами

*   **iOS/macOS**: Добавьте учетную запись типа "Другая" -> "Учетная запись CalDAV/CardDAV".
*   **Android**: Используйте приложение DAVx5.
    *   Тип входа: URL и логин/пароль.
    *   Базовый URL: `http://<ваш-ip>:8080/dav/`
*   **Thunderbird**: Встроенная поддержка CalDAV/CardDAV при настройке учетной записи.
*   **Matrix Client (Element)**: Требуется мост (bridge) или виджет, использующий данный API для отображения календаря в комнате Matrix.

## Разработка и Тесты

Проект покрыт тестами. Для запуска:

```bash
# Запуск всех тестов
go test ./...

# Запуск тестов с выводом покрытия
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Для локального тестирования RQLite в тестах используется временный инстанс в памяти или Docker-контейнер (см. `internal/store/test_helpers.go`).

## Производительность и Масштабирование

1.  **RQLite Cluster**: Для повышения отказоустойчивости запустите несколько узлов RQLite. Сервер автоматически подключится к лидеру кластера.
2.  **Stateless Backend**: Сам Go-сервер не хранит состояния. Вы можете запустить несколько экземпляров сервера за балансировщиком нагрузки (Nginx, HAProxy).
3.  **Вложения**: Большие бинарные вложения писем рекомендуется хранить в объектном хранилище (S3), сохраняя в RQLite только метаданные и ссылки (в текущей реализации для простоты все хранится в БД, но архитектура позволяет вынести это в отдельный интерфейс `BlobStore`).

## Лицензия

MIT License.

## Примечание для разработчиков

Код написан с упором на читаемость и простоту поддержки.
*   Используется стандартный пакет `database/sql` с драйвером `github.com/rqlite/gorqlite`.
*   XML парсинг для WebDAV реализован через `encoding/xml`.
*   Логирование осуществляется через `log/slog` (стандарт Go 1.21+).

При расширении функционала (например, добавление поддержки шифрования PGP или глубокой интеграции с Matrix Synapse через Admin API) следуйте структуре пакета `/internal`.
