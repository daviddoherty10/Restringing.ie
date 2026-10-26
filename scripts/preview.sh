#!/bin/sh
set -eu
script_dir=$(dirname "$0")
cd "$script_dir/.."

PREVIEW_DB_PORT=${PREVIEW_DB_PORT:-5544}
case "$PREVIEW_DB_PORT" in ''|*[!0-9]*) echo 'PREVIEW_DB_PORT must be a port number.' >&2; exit 1 ;; esac
if [ "$PREVIEW_DB_PORT" -lt 1 ] || [ "$PREVIEW_DB_PORT" -gt 65535 ]; then
  echo 'PREVIEW_DB_PORT must be 1–65535.' >&2
  exit 1
fi
export PREVIEW_DB_PORT
preview_url="postgres://restringing_preview:preview_local_only@127.0.0.1:${PREVIEW_DB_PORT}/restringing_preview?sslmode=disable"

case "${1:-}" in
  seed)
    command -v go >/dev/null
    command -v goose >/dev/null
    command -v docker >/dev/null
    go -C backend run ./cmd/preview validate -database-url "$preview_url"
    docker compose -f compose.preview.yaml up -d --wait db
    goose -dir backend/migrations postgres "$preview_url" up
    go -C backend run ./cmd/preview seed -database-url "$preview_url"
    ;;
  api)
    go -C backend run ./cmd/preview status -database-url "$preview_url"
    exec env DATABASE_URL="$preview_url" API_ADDR=127.0.0.1:8082 FRONTEND_ORIGIN=http://127.0.0.1:5174 go -C backend run ./cmd/api
    ;;
  web)
    cd frontend
    exec env VITE_API_BASE_URL=http://127.0.0.1:8082/api/v1 npm run dev -- --host 127.0.0.1 --port 5174 --strictPort
    ;;
  session)
    exec go -C backend run ./cmd/preview session -database-url "$preview_url" -role "${2:-customer}" -browser
    ;;
  *)
    echo 'Usage: bash scripts/preview.sh seed|api|web|session [customer|stringer|staff|admin]' >&2
    exit 1
    ;;
esac
