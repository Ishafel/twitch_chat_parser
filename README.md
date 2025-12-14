# Twitch Chat Logger

Программа для логирования чата Twitch в PostgreSQL.

## Стек технологий
- Go 1.22
- Docker
- PostgreSQL
- [github.com/gempir/go-twitch-irc/v4](https://github.com/gempir/go-twitch-irc/v4)
- [github.com/jackc/pgx/v5](https://github.com/jackc/pgx/v5)

## Установка и запуск

### Предварительные требования
- Docker
- Docker Compose

### Настройка
1. Скопируйте `.env.example` в `.env` и заполните переменные окружения:
   ```bash
   cp .env.example .env
   ```

2. Основные переменные окружения:
   - `TWITCH_CHANNEL` - имя канала Twitch для логирования
   - `TWITCH_USERNAME` - имя пользователя для подключения к чату
   - `TWITCH_OAUTH` - OAuth токен для доступа к чату
   - `DATABASE_URL` - строка подключения к PostgreSQL

### Запуск
```bash
# Сборка и запуск сервисов
docker-compose up --build

# Запуск только приложения (если база уже запущена)
docker-compose up --build app
```

## Структура проекта
```
app/          # Исходный код на Go
├── main.go   # Основной файл приложения

db/           # Конфигурация базы данных

docker-compose.yml  # Оркестрация сервисов
Dockerfile          # Сборка образа приложения
```

## Разработка

### Запуск локально (без Docker)
```bash
# Установка зависимостей
go mod tidy

# Запуск приложения
go run app/main.go
```

### Сборка

```bash
## Деплой докера в разработке
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build
```

```bash
# Сборка бинарного файла
CGO_ENABLED=0 go build -o app/app app/main.go
```

## Переменные окружения

| Переменная | Описание | Обязательная |
|------------|---------|-------------|
| TWITCH_CHANNEL | Канал Twitch для логирования | Да |
| TWITCH_USERNAME | Имя пользователя для подключения | Да |
| TWITCH_OAUTH | OAuth токен для доступа к чату | Да |
| DATABASE_URL | Строка подключения к PostgreSQL | Да |
| LOG_LEVEL | Уровень логирования (info, debug, error) | Нет |

## Лицензия
MIT
