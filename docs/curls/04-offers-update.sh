#!/usr/bin/env bash
# PATCH /api/v1/offers/{uuid} — все поля опциональны (partial update); вся
# матрица вариаций `flights` (issue №20): ключ отсутствует / пустой список /
# новый список.
#
# Использование:
#   TOKEN=eyJ... OFFER_UUID=<uuid из 03-offers-create.sh> bash docs/curls/04-offers-update.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
: "${TOKEN:?TOKEN обязателен (роль ROLE_AGENT/ROLE_SUPER_ADMIN) — см. docs/curls/README.md}"
: "${OFFER_UUID:?OFFER_UUID обязателен — возьмите uuid из ответа docs/curls/03-offers-create.sh}"

AUTH=(-H "Authorization: Bearer ${TOKEN}" -H 'Content-Type: application/json')

# ===== Только title/status, ключ flights отсутствует → 200, существующие =====
# ===== flights офера НЕ трогаются                                       =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X PATCH "${BASE_URL}/api/v1/offers/${OFFER_UUID}" \
  "${AUTH[@]}" \
  -d '{"title": "Тур в Париж (обновлено)", "status": "ready"}'

# ===== flights: [] → 200, все перелёты офера удаляются (если были) =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X PATCH "${BASE_URL}/api/v1/offers/${OFFER_UUID}" \
  "${AUTH[@]}" \
  -d '{"flights": []}'

# ===== flights с новым набором → 200, старые flights полностью заменяются =====
# ===== новыми в одной транзакции (diff-then-replace)                     =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X PATCH "${BASE_URL}/api/v1/offers/${OFFER_UUID}" \
  "${AUTH[@]}" \
  -d '{
    "flights": [
      {"segments": [{"departure_airport_icao": "UUEE", "arrival_airport_icao": "EDDF", "departure_at": "2026-09-10T09:00:00Z", "arrival_at": "2026-09-10T12:00:00Z"}]}
    ]
  }'

# ===== Повторный PATCH с идентичными flights (тот же порядок/аэропорты/ =====
# ===== таймстемпы) → 200, no-op — БД не трогается (см. код 04-выше)     =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X PATCH "${BASE_URL}/api/v1/offers/${OFFER_UUID}" \
  "${AUTH[@]}" \
  -d '{
    "flights": [
      {"segments": [{"departure_airport_icao": "UUEE", "arrival_airport_icao": "EDDF", "departure_at": "2026-09-10T09:00:00Z", "arrival_at": "2026-09-10T12:00:00Z"}]}
    ]
  }'

# ===== Невалидная хронология в flights → 400, остальные поля (title и =====
# ===== т. п.) тоже не применяются — весь PATCH атомарен               =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X PATCH "${BASE_URL}/api/v1/offers/${OFFER_UUID}" \
  "${AUTH[@]}" \
  -d '{
    "title": "Не должно примениться",
    "flights": [
      {"segments": [{"departure_airport_icao": "UUEE", "arrival_airport_icao": "LFPG", "departure_at": "2026-09-01T14:00:00Z", "arrival_at": "2026-09-01T10:00:00Z"}]}
    ]
  }'

# ===== Невалидный status → 400 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X PATCH "${BASE_URL}/api/v1/offers/${OFFER_UUID}" \
  "${AUTH[@]}" \
  -d '{"status": "archived"}'

# ===== Пустое тело ({}) → 200, ничего не меняется (все поля опциональны) =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X PATCH "${BASE_URL}/api/v1/offers/${OFFER_UUID}" \
  "${AUTH[@]}" \
  -d '{}'

# ===== Несуществующий/чужой uuid → 404 (не раскрываем существование =====
# ===== чужого offer)                                                =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X PATCH "${BASE_URL}/api/v1/offers/00000000-0000-0000-0000-000000000000" \
  "${AUTH[@]}" \
  -d '{"title": "x"}'

# ===== Без токена → 401 =====
curl -sS -w '\n--- HTTP %{http_code} ---\n' \
  -X PATCH "${BASE_URL}/api/v1/offers/${OFFER_UUID}" \
  -H 'Content-Type: application/json' \
  -d '{"title": "x"}'
