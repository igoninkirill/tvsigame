# Деплой «Своей Игры» на сервер

Пошаговая инструкция: от чистого VPS до работающей игры на своём домене с HTTPS.
Подходит любой VPS с KVM от 1 vCPU / 1 ГБ RAM / 15 ГБ диска (Ubuntu 24.04 рекомендуется).

## 0. Что понадобится

- VPS с публичным IPv4 (например, REG.RU Free Tier, FirstVDS, VDSina).
- Домен (например, `krutokir.online`) с возможностью править DNS.
- Этот репозиторий на локальной машине.

## 1. DNS

В панели управления доменом создайте A-запись:

```
krutokir.online  →  A  →  <IP сервера>
```

Дождитесь, пока запись разойдётся (`ping krutokir.online` с любой машины).

## 2. Подготовка сервера (один раз)

```bash
ssh root@<IP сервера>

# Docker + compose-плагин
curl -fsSL https://get.docker.com | sh
apt-get update && apt-get install -y docker-compose-plugin

# (опционально) не-root пользователь для приложения
```

## 3. Первый деплой

С локальной машины, из корня проекта:

```bash
# 3.1. Залить код (rsync поверх SSH)
SERVER=root@<IP сервера> ./scripts/deploy.sh
```

Затем на сервере создать `.env` (один раз):

```bash
ssh root@<IP сервера>
cd /opt/sigame
cat > .env <<'EOF'
DOMAIN=krutokir.online
POSTGRES_USER=sigame
POSTGRES_PASSWORD=<придумайте длинный пароль>
POSTGRES_DB=sigame
EOF

# 3.2. Запуск
docker compose -f docker-compose.prod.yml up -d --build
```

Через 1–2 минуты (сборка образа) игра доступна на `https://krutokir.online`.
Caddy сам выпустит сертификат Let's Encrypt — важно, чтобы DNS из шага 1
уже работал и порты 80/443 были открыты (в REG.RU/FirstVDS открыты по умолчанию).

## 4. Обновления

```bash
SERVER=root@<IP сервера> ./scripts/deploy.sh
```

Скрипт синхронизирует код, пересоберёт образ и перезапустит контейнеры.
**База и загруженные пакеты сохраняются** (Docker-тома `pgdata` и `media`).
Активные игры при перезапуске уходят в паузу и продолжаются с того же места.

## 5. Эксплуатация

```bash
# Логи
docker compose -f docker-compose.prod.yml logs -f app

# Бэкап базы (комнаты, счета, библиотека пакетов)
docker compose -f docker-compose.prod.yml exec db \
  pg_dump -U sigame sigame | gzip > backup-$(date +%F).sql.gz

# Бэкап медиа пакетов
docker run --rm -v sigame_media:/data -v "$PWD":/backup alpine \
  tar czf /backup/media-$(date +%F).tar.gz -C /data .
```

## 6. Если что-то не так

- `https://` не открывается → проверьте DNS (шаг 1) и что контейнеры живы:
  `docker compose -f docker-compose.prod.yml ps`
- Сертификат не выпускается → смотрите `logs caddy`: чаще всего DNS ещё не
  разошёлся или 80/443 закрыты файрволом.
- Белый экран/ошибки API → `logs app`; база не поднялась → `logs db`.

## Альтернатива без сервера

Если ТВ и смартфон ведущего в одной Wi-Fi сети с компьютером, где запущен
проект, интернет-сервер не нужен: `docker compose up -d` локально, и все
открывают `http://<IP компьютера в локальной сети>:8080`.
