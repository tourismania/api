#!/usr/bin/env bash
# DELETE /api/v1/offers/{uuid} — идёт последним, т. к. необратимо удаляет
# offer, созданный в docs/curls/03-offers-create.sh (soft delete).
#
# Использование:
#   TOKEN=eyJ... OFFER_UUID=<uuid из 03-offers-create.sh> bash docs/curls/06-offers-delete.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
: "${TOKEN:?TOKEN обязателен (роль ROLE_AGENT/ROLE_SUPER_ADMIN) — см. docs/curls/README.md}"
: "${OFFER_UUID:?OFFER_UUID обязателен — возьмите uuid из ответа docs/curls/03-offers-create.sh}"

AUTH=(-H "Authorization: Bearer ${TOKEN}")

# ===== Удаление своего offer → 204, тело пустое =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X DELETE "${BASE_URL}/api/v1/offers/${OFFER_UUID}" \
  "${AUTH[@]}"

# ===== Повторное удаление того же offer → 404 (soft-deleted offer уже =====
# ===== не виден)                                                     =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X DELETE "${BASE_URL}/api/v1/offers/${OFFER_UUID}" \
  "${AUTH[@]}"

# ===== Несуществующий offer → 404 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X DELETE "${BASE_URL}/api/v1/offers/00000000-0000-0000-0000-000000000000" \
  "${AUTH[@]}"

# ===== Без токена → 401 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X DELETE "${BASE_URL}/api/v1/offers/${OFFER_UUID}"
