# Twitch Chat Logger

Утилита для сбора сообщений чата Twitch и сохранения их в PostgreSQL. Приложение рассчитано на непрерывную работу в Docker, пишет сообщения батчами, устойчиво к временным сбоям соединения и предоставляет готовую схему БД для аналитики.

## Основные возможности
- Подключение к одному или нескольким каналам Twitch через IRC API.
- Буферизация сообщений и вставка пачками (до 100 строк или каждые ~1.5 секунды) для снижения нагрузки на базу.
- Автоматическое повторное подключение клиента Twitch при обрывах.
- Запись метаданных: ID сообщения, канал, идентификатор пользователя, никнеймы, бэйджи, цвет ника, статусы модератора/подписчика, количество битсов, время отправки и получения.
- Готовая миграция PostgreSQL (`db/init.sql`) с таблицей `chat_messages` и вьюхой `v_last_messages`.

## Стек
- Go 1.22
- Docker + Docker Compose
- PostgreSQL
- [gempir/go-twitch-irc](https://github.com/gempir/go-twitch-irc/v4)
- [jackc/pgx](https://github.com/jackc/pgx/v5)

## Быстрый старт в Docker

1. Создайте файл `.env` в корне репозитория и заполните переменные (см. таблицу ниже). Минимальный пример:
   ```env
   TWITCH_USERNAME=mybot
   TWITCH_OAUTH_TOKEN=oauth:xxxxxxxxxxxxxxxxxxxxxx
   TWITCH_CHANNELS=channel_one,channel_two

   POSTGRES_HOST=db
   POSTGRES_PORT=5432
   POSTGRES_DB=twitch
   POSTGRES_USER=postgres
   POSTGRES_PASSWORD=postgres
   ```

2. Запустите приложение и базу в режиме разработки:
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build
   ```
   Файл `db/init.sql` автоматически создаст таблицы и индексы. База станет доступна на `localhost:5432`.

3. Для остановки и удаления контейнеров:
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.dev.yml down -v
   ```

Если у вас уже развернута база, можно запускать только приложение:
```bash
docker compose up --build app
```

## Ручной запуск (без Docker)
```bash
export TWITCH_USERNAME=...              # имя бота/аккаунта
export TWITCH_OAUTH_TOKEN=oauth:...     # токен IRC, начинается с "oauth:"
export TWITCH_CHANNELS=channel1,channel2
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_DB=twitch
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres

cd app
go mod tidy
go run main.go
```

## Структура проекта
```
app/                  # Go-код приложения
├── main.go           # Логика подключения к Twitch и записи в БД
├── go.mod            # Модуль и зависимости

db/
└── init.sql          # Скрипт создания таблиц и вьюхи

Dockerfile            # Многоэтапная сборка бинаря
docker-compose.yml    # Контейнер приложения (ожидает .env)
docker-compose.dev.yml# База данных + volume для разработки
```

## Переменные окружения

| Переменная | Описание | Обязательная |
|------------|----------|--------------|
| `TWITCH_USERNAME` | Имя пользователя, от которого идёт подключение к чату | Да |
| `TWITCH_OAUTH_TOKEN` | OAuth-токен вида `oauth:...` | Да |
| `TWITCH_CHANNELS` | Список каналов через запятую (без `#`) | Да |
| `POSTGRES_HOST` | Хост PostgreSQL | Да |
| `POSTGRES_PORT` | Порт PostgreSQL | Да |
| `POSTGRES_DB` | Имя базы | Да |
| `POSTGRES_USER` | Пользователь базы | Да |
| `POSTGRES_PASSWORD` | Пароль пользователя | Да |

### Как получить Twitch OAuth токен для IRC
1. Зайдите на https://twitchapps.com/tmi/ под нужным аккаунтом (создайте отдельный бот-аккаунт, если не хотите использовать
   основной).
2. Нажмите **Connect** → подтвердите права → скопируйте сгенерированную строку вида `oauth:xxxxxxxxxxxxxxxxxxxx`.
3. Вставьте её в переменную `TWITCH_OAUTH_TOKEN` (в `.env` или окружении). Токен специфичен для IRC и не подходит для REST API.

## Что создаётся в базе
- Таблица `chat_messages` с уникальным `message_id`, временными метками отправки (`sent_at`) и приёма (`received_at`), индексом по `(channel, sent_at)` для быстрых выборок по каналу и диапазону времени.
- Вьюха `v_last_messages`, сортирующая сообщения в порядке убывания времени/ID для простого чтения последних строк.

## Полезные команды
- Сборка бинаря без Docker: `CGO_ENABLED=0 go build -o app/app app/main.go`.
- Проверка статуса контейнеров: `docker compose ps`.
- Просмотр логов приложения: `docker compose logs -f app`.

## Лицензия
MIT
