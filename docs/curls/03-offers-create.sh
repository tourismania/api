#!/usr/bin/env bash
# POST /api/v1/offers — базовые сценарии + вся матрица вариаций `flights`
# (issue №20). TOKEN должен принадлежать пользователю с ROLE_AGENT/
# ROLE_SUPER_ADMIN (см. "Ограничение: роли" в docs/curls/README.md) — иначе
# все сценарии, кроме "недостаточная роль", вернут 403 вместо ожидаемого кода.
#
# Использование:
#   TOKEN=eyJ... bash docs/curls/03-offers-create.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
: "${TOKEN:?TOKEN обязателен (роль ROLE_AGENT/ROLE_SUPER_ADMIN) — см. docs/curls/README.md}"

AUTH=(-H "Authorization: Bearer ${TOKEN}" -H 'Content-Type: application/json')

# ===== Без flights → 201, ответ {id, uuid} =====
# Скопируйте uuid из ответа в OFFER_UUID для 04/05/06-*.sh.
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  "${AUTH[@]}" \
  -d '{
    "title": "Тур в Париж",
    "description": "5 дней / 4 ночи, завтраки включены",
    "status": "draft"
  }'

# ===== С одним прямым перелётом (без пересадки) → 201, offer + flight в =====
# ===== одной транзакции                                                =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  "${AUTH[@]}" \
  -d '{
    "title": "Тур в Париж (прямой рейс)",
    "description": "5 дней / 4 ночи",
    "status": "draft",
    "flights": [
      {
        "segments": [
          {"departure_airport_icao": "UUEE", "arrival_airport_icao": "LFPG", "departure_at": "2026-09-01T10:00:00Z", "arrival_at": "2026-09-01T14:00:00Z"}
        ]
      }
    ]
  }'

# ===== С перелётом через пересадку (2 сегмента) → 201; total_duration_ =====
# ===== seconds/layovers будут видны при GET /offers/{uuid}             =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  "${AUTH[@]}" \
  -d '{
    "title": "Тур в Париж (через Домодедово)",
    "description": "5 дней / 4 ночи",
    "status": "draft",
    "flights": [
      {
        "segments": [
          {"departure_airport_icao": "UUEE", "arrival_airport_icao": "UUDD", "departure_at": "2026-09-01T10:00:00Z", "arrival_at": "2026-09-01T12:00:00Z"},
          {"departure_airport_icao": "UUDD", "arrival_airport_icao": "LFPG", "departure_at": "2026-09-01T13:00:00Z", "arrival_at": "2026-09-01T16:00:00Z"}
        ]
      }
    ]
  }'

# ===== Несколько независимых flights в одном offer → 201 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  "${AUTH[@]}" \
  -d '{
    "title": "Тур в Париж туда-обратно",
    "description": "5 дней / 4 ночи",
    "status": "draft",
    "flights": [
      {"segments": [{"departure_airport_icao": "UUEE", "arrival_airport_icao": "LFPG", "departure_at": "2026-09-01T10:00:00Z", "arrival_at": "2026-09-01T14:00:00Z"}]},
      {"segments": [{"departure_airport_icao": "LFPG", "arrival_airport_icao": "UUEE", "departure_at": "2026-09-06T18:00:00Z", "arrival_at": "2026-09-06T23:00:00Z"}]}
    ]
  }'

# ===== Невалидная хронология сегмента (arrival_at <= departure_at) → 400; =====
# ===== offer НЕ создаётся вообще (валидация до открытия транзакции)      =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  "${AUTH[@]}" \
  -d '{
    "title": "Невалидный тур",
    "description": "не должен создаться",
    "status": "draft",
    "flights": [
      {"segments": [{"departure_airport_icao": "UUEE", "arrival_airport_icao": "LFPG", "departure_at": "2026-09-01T14:00:00Z", "arrival_at": "2026-09-01T10:00:00Z"}]}
    ]
  }'

# ===== Разрыв маршрута: аэропорт прилёта сегмента 1 != аэропорту вылета =====
# ===== сегмента 2 → 400                                                 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  "${AUTH[@]}" \
  -d '{
    "title": "Невалидный тур (разрыв маршрута)",
    "description": "не должен создаться",
    "status": "draft",
    "flights": [
      {
        "segments": [
          {"departure_airport_icao": "UUEE", "arrival_airport_icao": "UUDD", "departure_at": "2026-09-01T10:00:00Z", "arrival_at": "2026-09-01T12:00:00Z"},
          {"departure_airport_icao": "EDDF", "arrival_airport_icao": "LFPG", "departure_at": "2026-09-01T13:00:00Z", "arrival_at": "2026-09-01T16:00:00Z"}
        ]
      }
    ]
  }'

# ===== Пересадка нулевой/отрицательной длительности (departure_at сегмента =====
# ===== N+1 <= arrival_at сегмента N) → 400                                =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  "${AUTH[@]}" \
  -d '{
    "title": "Невалидный тур (нулевая пересадка)",
    "description": "не должен создаться",
    "status": "draft",
    "flights": [
      {
        "segments": [
          {"departure_airport_icao": "UUEE", "arrival_airport_icao": "UUDD", "departure_at": "2026-09-01T10:00:00Z", "arrival_at": "2026-09-01T12:00:00Z"},
          {"departure_airport_icao": "UUDD", "arrival_airport_icao": "LFPG", "departure_at": "2026-09-01T12:00:00Z", "arrival_at": "2026-09-01T16:00:00Z"}
        ]
      }
    ]
  }'

# ===== Несуществующий icao → 400 (ErrFlightAirportNotFound) =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  "${AUTH[@]}" \
  -d '{
    "title": "Невалидный тур (несуществующий аэропорт)",
    "description": "не должен создаться",
    "status": "draft",
    "flights": [
      {"segments": [{"departure_airport_icao": "ZZZZ", "arrival_airport_icao": "LFPG", "departure_at": "2026-09-01T10:00:00Z", "arrival_at": "2026-09-01T14:00:00Z"}]}
    ]
  }'

# ===== Пустой список сегментов внутри flight → 400 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  "${AUTH[@]}" \
  -d '{
    "title": "Невалидный тур (пустые segments)",
    "description": "не должен создаться",
    "status": "draft",
    "flights": [{"segments": []}]
  }'

# ===== Невалидное тело offer (пустой title, недопустимый status) → 400 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  "${AUTH[@]}" \
  -d '{"title": "", "description": "x", "status": "unknown-status"}'

# ===== Недостаточная роль (свежесозданный пользователь, ROLE_USER) → 403 =====
# Используйте TOKEN пользователя без ROLE_AGENT/ROLE_SUPER_ADMIN, например
# сразу после docs/curls/01-users.sh + docs/curls/00-auth.sh.
if [[ -n "${USER_TOKEN:-}" ]]; then
  curl -sS -w '\n--- HTTP %{http_code} ---\n' \
    -X POST "${BASE_URL}/api/v1/offers" \
    -H "Authorization: Bearer ${USER_TOKEN}" -H 'Content-Type: application/json' \
    -d '{"title": "Недоступно ROLE_USER", "description": "x", "status": "draft"}'
else
  echo 'USER_TOKEN не задан — пропускаю сценарий 403 (нужен токен пользователя без ROLE_AGENT/ROLE_SUPER_ADMIN)'
fi

# ===== Без токена → 401 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/offers" \
  -H 'Content-Type: application/json' \
  -d '{"title": "x", "description": "x", "status": "draft"}'
