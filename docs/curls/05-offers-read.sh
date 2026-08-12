#!/usr/bin/env bash
# GET /api/v1/offers (список), GET /api/v1/offers/{uuid}, GET /api/v1/public/offers/{uuid}
#
# Использование:
#   TOKEN=eyJ... OFFER_UUID=<uuid из 03-offers-create.sh> bash docs/curls/05-offers-read.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
: "${TOKEN:?TOKEN обязателен — см. docs/curls/00-auth.sh}"
: "${OFFER_UUID:?OFFER_UUID обязателен — возьмите uuid из ответа docs/curls/03-offers-create.sh}"

AUTH=(-H "Authorization: Bearer ${TOKEN}")

# ===== Список offer своего агентства, без фильтров → 200; flights в =====
# ===== ответе НЕТ (список — облегчённая проекция)                   =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${AUTH[@]}" \
  "${BASE_URL}/api/v1/offers"

# ===== Список с фильтром по статусу и пагинацией → 200 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${AUTH[@]}" \
  -G "${BASE_URL}/api/v1/offers" \
  --data-urlencode 'status=draft' \
  --data-urlencode 'limit=10' \
  --data-urlencode 'offset=0'

# ===== Список с невалидным status → 400 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${AUTH[@]}" \
  -G "${BASE_URL}/api/v1/offers" \
  --data-urlencode 'status=not-a-status'

# ===== Один offer своего агентства → 200, вместе с flights (сегменты, =====
# ===== total_duration_seconds, layovers — вычислено на лету)          =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${AUTH[@]}" \
  "${BASE_URL}/api/v1/offers/${OFFER_UUID}"

# ===== Несуществующий/чужой offer → 404 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${AUTH[@]}" \
  "${BASE_URL}/api/v1/offers/00000000-0000-0000-0000-000000000000"

# ===== Публичная ссылка на published offer → 200, БЕЗ авторизации =====
# offer должен быть предварительно переведён в status=published через
# docs/curls/04-offers-update.sh (-d '{"status":"published"}').
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${BASE_URL}/api/v1/public/offers/${OFFER_UUID}"

# ===== Публичная ссылка на draft/ready offer → 404 (не раскрываем =====
# ===== существование неопубликованного offer)                    =====
# Работает "из коробки" сразу после docs/curls/03-offers-create.sh, пока
# offer ещё не переведён в published.
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${BASE_URL}/api/v1/public/offers/${OFFER_UUID}"

# ===== Список без токена → 401 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  "${BASE_URL}/api/v1/offers"
