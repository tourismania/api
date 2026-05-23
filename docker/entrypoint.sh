#!/bin/sh
# entrypoint.sh — production container startup script.
#
# Runs database migrations before handing off to the server binary.
# If migrations fail the script exits with a non-zero code, which causes
# Docker to restart the container according to the `restart` policy.
#
# `set -e` — exit immediately on any error.
# `exec`   — replace the shell process with the server so that PID 1 is
#             the server and OS signals (SIGTERM, SIGINT) are delivered
#             directly to it without an extra shell wrapper.
set -e

DB_URL="postgres://${DATABASE_USER}:${DATABASE_PASSWORD}@${DATABASE_HOST}:${DATABASE_PORT}/${DATABASE_NAME}?sslmode=${DATABASE_SSLMODE}"

echo "[entrypoint] running database migrations..."
/app/migrate -path /app/migrations/postgres -database "$DB_URL" up
echo "[entrypoint] migrations applied, starting server..."

exec "$@"
