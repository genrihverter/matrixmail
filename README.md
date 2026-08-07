# Matrix Mail Gateway

Сервер уведомлений на Go, который пересылает сообщения из чатов Matrix на email через SMTP relay (например, mail.ru).

## Особенности

*   **Мониторинг комнат Matrix**: Сервер подключается к Matrix homeserver и отслеживает новые сообщения в комнатах.
*   **SMTP Relay**: Отправка уведомлений на email через внешний SMTP сервер с поддержкой TLS/STARTTLS.
*   **Гибкое маппирование**: Настройка соответствий между пользователями Matrix и email адресами.
*   **Фильтрация по комнатам**: Возможность подписки на уведомления из конкретных комнат или всех комнат.
*   **HTML и текстовые уведомления**: Письма содержат как текстовую, так и HTML версию сообщения.
*   **Graceful Shutdown**: Корректная остановка сервиса при получении сигнала завершения.

## Архитектура

```text
/cmd
  /server         # Точка входа приложения
/internal
  /config         # Конфигурация приложения (JSON)
  /matrix         # Клиент для работы с Matrix API
  /notifier       # Логика нотификации (синхронизация и отправка)
  /smtprelay      # SMTP клиент для отправки email
```

## Требования

*   **Go**: версия 1.19 или выше.
*   **Matrix Account**: Аккаунт в Matrix с access token для бота.
*   **SMTP Relay**: Доступ к SMTP серверу (например, mail.ru, gmail.com).

## Быстрый старт

### 1. Создание конфигурации

Создайте файл `config.json` в корне проекта:

```json
{
  "smtp_port": 2525,
  "imap_port": 143,
  "domain": "localhost",
  "storage_path": "./maildata",
  "smtp_relay": {
    "host": "smtp.mail.ru",
    "port": 587,
    "username": "your_email@mail.ru",
    "password": "your_app_password",
    "from_address": "your_email@mail.ru",
    "from_name": "Matrix Bot",
    "use_tls": false,
    "use_starttls": true
  },
  "matrix_user_mappings": [
    {
      "matrix_user_id": "@user1:matrix.org",
      "email": "user1@example.com",
      "room_ids": ["!roomid1:matrix.org"],
      "enabled": true
    },
    {
      "matrix_user_id": "@user2:matrix.org",
      "email": "user2@gmail.com",
      "room_ids": [],
      "enabled": true
    }
  ],
  "matrix": {
    "homeserver_url": "https://matrix.org",
    "access_token": "your_access_token_here",
    "user_id": "@botname:matrix.org",
    "enabled": true,
    "request_timeout": 30,
    "reconnect_delay": 10
  }
}
```

**Важно**: Для mail.ru необходимо использовать [пароль приложения](https://help.mail.ru/mail-help/security/2fa), а не основной пароль от почты.

### 2. Получение Matrix Access Token

1.  Войдите в свой аккаунт Matrix через Element или другой клиент.
2.  Перейдите в настройки аккаунта -> Help & About -> Advanced -> Access Token.
3.  Скопируйте токен и укажите его в конфигурации.

Или используйте API:

```bash
curl -X POST 'https://matrix.org/_matrix/client/v3/login' \
  -H 'Content-Type: application/json' \
  --data-raw '{
    "type": "m.login.password",
    "identifier": {"type": "m.id.user", "user": "your_username"},
    "password": "your_password"
  }'
```

### 3. Установка зависимостей и запуск

```bash
# Установка зависимостей
go mod tidy

# Запуск сервера
go run ./cmd/server -config config.json
```

## Использование

### Конфигурация maппирования пользователей

В секции `matrix_user_mappings` указываются пользователи, которые будут получать уведомления:

*   `matrix_user_id`: UserID в Matrix (например, `@username:matrix.org`).
*   `email`: Email адрес для получения уведомлений.
*   `room_ids`: Список комнат для мониторинга. Если пустой (`[]`) — уведомления приходят из всех комнат, где участвует бот. Можно указать `"*"` для всех комнат.
*   `enabled`: Включить или выключить уведомления для этого пользователя.

### Примеры сценариев

**Сценарий 1: Личные уведомления**
Каждый пользователь получает уведомления о всех сообщениях в комнатах, где он состоит:

```json
{
  "matrix_user_mappings": [
    {
      "matrix_user_id": "@alice:matrix.org",
      "email": "alice@example.com",
      "room_ids": [],
      "enabled": true
    }
  ]
}
```

**Сценарий 2: Уведомления из конкретной комнаты**
Пользователь получает уведомления только из указанных комнат:

```json
{
  "matrix_user_mappings": [
    {
      "matrix_user_id": "@bob:matrix.org",
      "email": "bob@example.com",
      "room_ids": ["!projectRoom:matrix.org"],
      "enabled": true
    }
  ]
}
```

**Сценарий 3: Несколько получателей**
Одно сообщение может быть отправлено нескольким пользователям:

```json
{
  "matrix_user_mappings": [
    {
      "matrix_user_id": "@user1:matrix.org",
      "email": "user1@company.com",
      "room_ids": ["!general:matrix.org"],
      "enabled": true
    },
    {
      "matrix_user_id": "@user2:matrix.org",
      "email": "user2@company.com",
      "room_ids": ["!general:matrix.org"],
      "enabled": true
    }
  ]
}
```

## Пример письма

Уведомление приходит в формате HTML и текста:

```
Тема: Сообщение в комнате "Общая" от Иван Иванов

От: Иван Иванов (@ivan:matrix.org)
Комната: Общая

Привет всем! Это тестовое сообщение.

---
Это автоматическое уведомление от Matrix Mail Gateway
```

## Разработка и тесты

```bash
# Запуск всех тестов
go test ./...

# Запуск тестов с покрытием
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Сборка бинарного файла
go build -o matrix-mail-gateway ./cmd/server
```

## Производительность

*   **Асинхронная отправка**:Email отправляются асинхронно в отдельных горутине, что не блокирует обработку новых сообщений Matrix.
*   **Кэширование имен**: Отображаемые имена пользователей и названия комнат кэшируются для уменьшения количества запросов к Matrix API.
*   **Long-polling Sync**: Используется механизм `/sync` API Matrix с long-polling для эффективного получения новых событий.

## Безопасность

*   **Access Token**: Храните access token в безопасном месте, не коммитьте его в репозиторий.
*   **SMTP Пароль**: Используйте пароль приложения вместо основного пароля.
*   **TLS/STARTTLS**: Всегда включайте шифрование при подключении к SMTP серверу.

## Структура проекта

```
mail-matrix-server/
├── cmd/
│   └── server/
│       └── main.go           # Точка входа
├── internal/
│   ├── config/
│   │   └── config.go         # Конфигурация и загрузка JSON
│   ├── matrix/
│   │   ├── client.go         # Matrix API клиент
│   │   └── client_test.go    # Тесты Matrix клиента
│   ├── notifier/
│   │   └── notifier.go       # Логика нотификации
│   └── smtprelay/
│       ├── relay.go          # SMTP relay клиент
│       └── relay_test.go     # Тесты SMTP клиента
├── config.example.json       # Пример конфигурации
├── go.mod                    # Go модуль
└── README.md                 # Документация
```

## Лицензия

MIT License.

## Troubleshooting

**Ошибка аутентификации SMTP**:
*   Проверьте логин и пароль (для mail.ru используйте пароль приложения).
*   Убедитесь, что `from_address` совпадает с `username`.
*   Проверьте порт (587 для STARTTLS, 465 для SMTPS).

**Ошибка подключения к Matrix**:
*   Проверьте `homeserver_url` (должен включать `https://`).
*   Убедитесь, что `access_token` действителен.
*   Проверьте сеть и фаервол.

**Уведомления не приходят**:
*   Проверьте `enabled: true` в настройках маппинга.
*   Убедитесь, что бот состоит в указанных комнатах.
*   Проверьте логи сервера на наличие ошибок.
