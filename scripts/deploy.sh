#!/usr/bin/env bash
# Деплой «Своей Игры» на продакшен-сервер по SSH.
#
# Использование:
#   SERVER=ubuntu@1.2.3.4 ./scripts/deploy.sh          # rsync + пересборка на сервере
#   SERVER=ubuntu@1.2.3.4 ./scripts/deploy.sh --logs   # посмотреть логи после деплоя
#
# Перед первым запуском на сервере должен быть установлен Docker
# (см. deploy/DEPLOY.md, шаг 2) и в корне проекта НА СЕРВЕРЕ создан .env
# с DOMAIN и POSTGRES_PASSWORD.

set -euo pipefail

SERVER="${SERVER:?Укажите SERVER=user@host}"
REMOTE_DIR="${REMOTE_DIR:-/opt/sigame}"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "== 1/3. Синхронизация проекта -> $SERVER:$REMOTE_DIR"
rsync -az --delete \
  --exclude '.git' \
  --exclude 'frontend/node_modules' \
  --exclude 'frontend/dist' \
  --exclude 'data/media' \
  --exclude 'tmp*' \
  --exclude '.env' \
  "$PROJECT_ROOT/" "$SERVER:$REMOTE_DIR/"

echo "== 2/3. Сборка и перезапуск на сервере"
ssh "$SERVER" "cd '$REMOTE_DIR' && docker compose -f docker-compose.prod.yml up -d --build"

echo "== 3/3. Проверка"
ssh "$SERVER" "curl -sf -o /dev/null -w 'app: %{http_code}\n' http://127.0.0.1/ || true"

if [[ "${1:-}" == "--logs" ]]; then
  ssh "$SERVER" "cd '$REMOTE_DIR' && docker compose -f docker-compose.prod.yml logs --tail=50"
fi

echo "Готово. Игра доступна по https://<DOMAIN из .env на сервере>"
