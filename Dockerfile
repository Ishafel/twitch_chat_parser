# ---------- build ----------
FROM golang:1.22 AS build
WORKDIR /src

# Сначала только модули — для кеша
COPY app/go.mod ./app/go.mod
# Если локально go.sum нет — этот COPY можно опустить.
# COPY app/go.sum ./app/go.sum

# Скачаем зависимости (создаст go.sum, если его не было)
RUN cd app && go mod download

# Теперь копируем весь код и подчищаем зависимости
COPY app/ ./app/
RUN cd app && go mod tidy

# Сборка (на M4 не фиксируем GOARCH!)
WORKDIR /src/app
RUN CGO_ENABLED=0 go build -o /out/app ./cmd/chat-logger

# ---------- runtime ----------
FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=build /out/app /app/app
USER nonroot:nonroot
ENTRYPOINT ["/app/app"]
