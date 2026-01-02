.PHONY: deps tidy build test docker-build compose-up compose-down clean help

APP_DIR := app
BIN_DIR := bin
BINARY := $(BIN_DIR)/app

help: ## Показать справку о целях (docker/compose при наличии читают .env и .env.extra*).
	@awk 'BEGIN {FS = ": .*## "} /^[A-Za-z0-9_.-]+:.*##/ {printf "%-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: tidy ## Запустить go mod tidy в app/ (алиас tidy; docker/compose читают .env и .env.extra*).

tidy: ## Запустить go mod tidy в app/ (привести зависимости; docker/compose читают .env и .env.extra*).
	cd $(APP_DIR) && go mod tidy

build: ## Собрать бинарник bin/app с отключённым CGO (docker/compose учитывают .env и .env.extra*).
	mkdir -p $(BIN_DIR)
	cd $(APP_DIR) && CGO_ENABLED=0 go build -o ../$(BINARY) .

test: ## Запустить Go-тесты из app/ (.env и .env.extra* используются, если нужны зависимостям).
	cd $(APP_DIR) && go test ./...

docker-build: ## Собрать образ twitch-chat-logger; docker учитывает .env и .env.extra*.
	docker build -f Dockerfile -t twitch-chat-logger .

compose-up: ## Скомбинировать базовый docker-compose.yml с dev-оверрайдом и поднять сервисы (.env и .env.extra*).
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build

compose-down: ## Остановить сервисы, собранные базой+dev-оверрайдом, и удалить volume'ы (.env и .env.extra*).
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down -v

clean: ## Удалить артефакты сборки вроде bin/app.
	rm -rf $(BIN_DIR)
