#!/usr/bin/env bash
# POST /api/v1/users, GET /api/v1/users/me
#
# Использование:
#   AGENCY_ID=1 bash docs/curls/01-users.sh          # регистрация не требует TOKEN
#   TOKEN=eyJ... bash docs/curls/01-users.sh          # для GET /users/me
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
AGENCY_ID="${AGENCY_ID:-1}"

# ===== Регистрация пользователя → 201 =====
# Новый пользователь получает пустой список ролей (эквивалент ROLE_USER) —
# см. "Ограничение: роли" в docs/curls/README.md.
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/users" \
  -H 'Content-Type: application/json' \
  -d "{
    \"first_name\": \"Ada\",
    \"last_name\": \"Lovelace\",
    \"email\": \"ada+$(date +%s)@example.com\",
    \"password\": \"secret123\",
    \"agency_id\": ${AGENCY_ID}
  }"

# ===== Дубликат email → 500/400 в зависимости от constraint-обработки =====
# (используйте email уже созданного пользователя)
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/users" \
  -H 'Content-Type: application/json' \
  -d "{
    \"first_name\": \"Ada\",
    \"last_name\": \"Lovelace\",
    \"email\": \"ada@example.com\",
    \"password\": \"secret123\",
    \"agency_id\": ${AGENCY_ID}
  }"

# ===== Несуществующее агентство → 400 (ErrAgencyNotFound) =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/users" \
  -H 'Content-Type: application/json' \
  -d '{
    "first_name": "Ada",
    "last_name": "Lovelace",
    "email": "ada.orphan@example.com",
    "password": "secret123",
    "agency_id": 999999
  }'

# ===== Невалидное тело (короткий пароль, agency_id <= 0) → 400 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/v1/users" \
  -H 'Content-Type: application/json' \
  -d '{
    "first_name": "Ada",
    "last_name": "Lovelace",
    "email": "not-an-email",
    "password": "123",
    "agency_id": 0
  }'

# ===== Профиль текущего пользователя → 200 (нужен TOKEN) =====
if [[ -n "${TOKEN:-}" ]]; then
  curl -sS -w '\n--- HTTP %{http_code} ---\n' \
    "${BASE_URL}/api/v1/users/me" \
    -H "Authorization: Bearer ${TOKEN}"
else
  echo 'TOKEN не задан — пропускаю GET /api/v1/users/me (см. docs/curls/00-auth.sh)'
fi

# ===== GET /api/v1/users/me без токена → 401 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${BASE_URL}/api/v1/users/me"
