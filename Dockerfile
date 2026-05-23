# syntax=docker/dockerfile:1.7
# Включает расширенный синтаксис BuildKit (кэш-маунты, heredoc и др.)

# ╔══════════════════════════════════════════════════════════╗
# ║  STAGE 1 — dev                                           ║
# ║  Образ для локальной разработки: hot-reload через air    ║
# ║  и удалённый отладчик Delve.                             ║
# ║  Исходники монтируются снаружи (docker-compose volume),  ║
# ║  поэтому COPY исходников здесь нет.                      ║
# ╚══════════════════════════════════════════════════════════╝
FROM golang:1.26-alpine AS dev

# Системные утилиты:
# make    — запуск целей из Makefile (generate, migrate, swag и т.д.).
# openssl — генерация RSA/EC-ключей для JWT и TLS-сертификатов локально.
RUN apk add --no-cache make openssl

# Go-инструменты для разработки:
# air     — hot-reload: пересобирает бинарник при изменении *.go.
# dlv     — Delve: удалённый отладчик (порт 2345), нужен для IDE remote debug.
# sqlc    — кодогенерация типобезопасных Go-структур из SQL-запросов.
# migrate — применение/откат SQL-миграций (тег pgx5 — pure-Go, без CGO).
# swag    — генерация Swagger/OpenAPI-документации из аннотаций в коде.
RUN go install github.com/air-verse/air@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest \
    && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
    && go install github.com/swaggo/swag/cmd/swag@latest

WORKDIR /app

# 8080 — HTTP сервер, 2345 — Delve remote debugger
EXPOSE 8080 2345

# air читает .air.toml из рабочей директории (/app — смотри volume в override).
ENTRYPOINT ["air", "-c", ".air.toml"]

# ╔══════════════════════════════════════════════════════════╗
# ║  STAGE 2 — builder                                       ║
# ║  Компилируем оба бинарника в изолированном Go-окружении. ║
# ║  В финальный образ этот слой не попадает.                ║
# ╚══════════════════════════════════════════════════════════╝
FROM golang:1.26-alpine AS builder

# Рабочая директория внутри builder-контейнера
WORKDIR /src

# Поставим make для унификации команд
RUN apk add --no-cache make

# Копируем только манифесты зависимостей — это позволяет Docker
# переиспользовать кэш слоя при изменениях в коде (но не в go.mod/go.sum).
COPY go.mod go.sum* ./
RUN go mod download

# Копируем исходники после загрузки зависимостей, чтобы не инвалидировать кэш.
COPY . .

RUN go install github.com/swaggo/swag/cmd/swag@latest

# Сгенерируем swagger-документацию
RUN make swag

# Собираем статически слинкованные бинарники (CGO_ENABLED=0), чтобы они
# запускались в минимальном alpine без libc.
# -trimpath  — убирает абсолютные пути из отладочной информации (безопасность).
# -ldflags="-s -w" — удаляет символы и DWARF-отладку, уменьшает размер бинарника.
# TARGETOS/TARGETARCH пробрасываются BuildKit при cross-компиляции (docker buildx).
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/cli    ./cmd/cli

# ╔══════════════════════════════════════════════════════════╗
# ║  STAGE 3 — runner                                        ║
# ║  Минимальный production-образ: только бинарники и        ║
# ║  необходимые runtime-файлы. Go-компилятор не включён.    ║
# ╚══════════════════════════════════════════════════════════╝
FROM alpine:3.20 AS runner

# ca-certificates — для HTTPS-запросов к внешним сервисам.
# tzdata         — для корректной работы time.LoadLocation (таймзоны).
# Создаём непривилегированного пользователя app — процесс не работает от root.
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S app -G app

WORKDIR /app

# Копируем артефакты из builder; chown сразу задаёт владельца без лишнего слоя.
COPY --from=builder --chown=app:app /out/server      /app/server
COPY --from=builder --chown=app:app /out/cli         /app/cli

COPY --from=dev    --chown=app:app /go/bin/migrate  /app/migrate
# SQL-миграции — запускаются entrypoint.sh перед стартом сервера.
COPY --from=builder --chown=app:app /src/migrations  /app/migrations

# Конфигурационные файлы (YAML/TOML/env-шаблоны) для разных окружений.
COPY --from=builder --chown=app:app /src/config      /app/config

# Документацию (swagger)
COPY --from=builder --chown=app:app /src/docs        /app/docs

# Entrypoint-скрипт: запускает миграции, затем передаёт управление серверу.
# chmod выполняется от root (до USER app), поэтому всегда успешен.
COPY --chown=app:app docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Документируем порт HTTP-сервера (фактически открывается в docker-compose/k8s).
EXPOSE 8080
# Переключаемся на непривилегированного пользователя перед запуском.
USER app

# Абсолютный путь — надёжнее в exec-форме, не зависит от WORKDIR.
# CMD передаётся в entrypoint как $@ — скрипт делает `exec "$@"`,
# заменяя себя сервером (PID 1 = /app/server).
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/server"]
