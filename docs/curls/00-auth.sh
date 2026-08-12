#!/usr/bin/env bash
# POST /api/login — сценарии успешного и неуспешного логина.
#
# Использование:
#   BASE_URL=http://localhost:8080 EMAIL=ada@example.com PASSWORD=secret bash docs/curls/00-auth.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
EMAIL="${EMAIL:-ada@example.com}"
PASSWORD="${PASSWORD:-secret}"

# ===== Успешный логин → 200, тело содержит JWT =====
# Токен из этого ответа положить в переменную TOKEN для остальных скриптов.
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}"

# ===== Неверный пароль → 401 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"definitely-wrong\"}"

# ===== Несуществующий email → 401 (тот же ответ, что и неверный пароль — =====
# ===== не раскрываем существование аккаунта)                           =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"no-such-user@example.com","password":"whatever"}'

# ===== Невалидное тело (нет password) → 400 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X POST "${BASE_URL}/api/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com"}'
