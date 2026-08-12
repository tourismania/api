#!/usr/bin/env bash
# GET /api/v1/airports — требует TOKEN и предварительно синхронизированный
# справочник (make ... / go run ./cmd/cli airports sync, см. README.md).
#
# Использование:
#   TOKEN=eyJ... bash docs/curls/02-airports.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
: "${TOKEN:?TOKEN обязателен — см. docs/curls/00-auth.sh}"

AUTH=(-H "Authorization: Bearer ${TOKEN}")

# ===== Успешный поиск по названию → 200 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${AUTH[@]}" \
  -G "${BASE_URL}/api/v1/airports" \
  --data-urlencode 'search=Moscow'

# ===== Поиск с пагинацией → 200, meta.limit/offset отражают параметры =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${AUTH[@]}" \
  -G "${BASE_URL}/api/v1/airports" \
  --data-urlencode 'search=airport' \
  --data-urlencode 'limit=5' \
  --data-urlencode 'offset=10'

# ===== search короче 2 символов → 400 (INVALID_SEARCH, structured error) =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${AUTH[@]}" \
  -G "${BASE_URL}/api/v1/airports" \
  --data-urlencode 'search=a'

# ===== search отсутствует → 400 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${AUTH[@]}" \
  "${BASE_URL}/api/v1/airports"

# ===== limit вне диапазона (>100) → 400 (validator) =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${AUTH[@]}" \
  -G "${BASE_URL}/api/v1/airports" \
  --data-urlencode 'search=airport' \
  --data-urlencode 'limit=1000'

# ===== Без токена → 401 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -G "${BASE_URL}/api/v1/airports" \
  --data-urlencode 'search=Moscow'
