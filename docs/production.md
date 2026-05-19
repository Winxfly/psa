# Production

Production-запуск backend-сервиса и инфраструктуры.

В этом режиме публичный HTTP/HTTPS-трафик принимает Caddy, а backend, PostgreSQL, Redis и observability-сервисы не публикуются наружу напрямую.

## Оглавление

- [Требования](#requirements)
- [Подготовка](#setup)
- [Production stack](#production-stack)
- [Миграции и администратор](#migrations-admin)
- [Схема доступа](#access)
- [Проверка API](#api-check)
- [PostgreSQL access](#db-access)
- [Observability access](#observability-access)
- [Остановка](#stop)
- [Frontend](#frontend)

<a id="requirements"></a>
## Требования

- Docker
- Docker Compose
- Make

<a id="setup"></a>
## Подготовка

Создать файл `.env` по примеру `.env.example`:

```bash
cp .env.example .env
```

`.env` обязателен. Без него приложение намеренно не стартует.

Для production-запуска необходимо отредактировать `.env` под свои нужды, например:

```env
CONFIG_PATH=/config/prod.yaml
APP_DOMAIN=example.com
CADDY_EMAIL=admin@example.com
CORS_ALLOWED_ORIGINS=https://example.com
DB_HOST_PORT=5433
PROMETHEUS_HOST_PORT=9090
GRAFANA_HOST_PORT=3001
```

Также нужно заменить значения секретов и паролей:

- `HH_CLIENT_ID`
- `HH_CLIENT_SECRET`
- `HH_USER_AGENT`
- `DB_PASSWORD`
- `JWT_SECRET`
- `GRAFANA_ADMIN_PASSWORD`

Если не указать ключи HH API, backend сможет стартовать, но сбор данных с hh.ru будет недоступен.

<a id="production-stack"></a>
## Production stack

Поднять production stack:

```bash
make prod-up
```

Команда поднимает:

- `caddy`
- `backend`
- `postgres`
- `redis`
- `prometheus`
- `grafana`
- `loki`
- `alloy`

Backend не публикуется наружу напрямую. Проверка API выполняется через Caddy.

<a id="migrations-admin"></a>
## Миграции и администратор

Если БД ещё не инициализирована, применить миграции:

```bash
make prod-migrate-up
```

Создать администратора:

```bash
make prod-create-admin ADMIN_EMAIL=admin@example.com ADMIN_PASSWORD=supersecret
```

Администратор нужен для закрытых API: управление профессиями и ручной запуск scraping. Наличие администратора необязательно.

<a id="access"></a>
## Схема доступа

Адреса могут быть изменены в `.env`.

Публично доступны:

- `80/tcp` - Caddy HTTP
- `443/tcp` - Caddy HTTPS

Доступны только на localhost сервера:

- PostgreSQL: `127.0.0.1:${DB_HOST_PORT:-5433}`
- Prometheus: `127.0.0.1:${PROMETHEUS_HOST_PORT:-9090}`
- Grafana: `127.0.0.1:${GRAFANA_HOST_PORT:-3001}`

Доступны только внутри Docker network:

- `backend:8080`
- `redis:6379`
- `loki:3100`

<a id="api-check"></a>
## Проверка API

Для реального домена:

```bash
curl https://example.com/health/ready
```

Ожидаемый ответ:

```json
{
  "status": "ok",
  "checks": {
    "db": "ok",
    "cache": "ok"
  }
}
```

Для локального smoke с `APP_DOMAIN=localhost`:

```bash
curl -k https://localhost/health/ready
```

`-k` нужен только для локального `https://localhost`, потому что сертификат может быть не доверен системой. Для реального домена `-k` не нужен.

[API examples](api.md)

<a id="db-access"></a>
## PostgreSQL access

Напрямую снаружи это недоступно, поэтому нужно использовать внутри сервера или SSH tunnel, например:

PostgreSQL:

```bash
ssh -L 5433:localhost:5433 <user>@<server>
```

После подключения, например, в pgAdmin:

- Host: `localhost`
- Port: `5433`
- Database: значение `DB_NAME`
- Username: значение `DB_USERNAME`
- Password: значение `DB_PASSWORD`

<a id="observability-access"></a>
## Observability access

Напрямую снаружи это недоступно, поэтому нужно использовать внутри сервера или SSH tunnel, например:

Grafana:

```bash
ssh -L 3001:localhost:3001 <user>@<server>
```

Prometheus:

```bash
ssh -L 9090:localhost:9090 <user>@<server>
```

Подробнее про dashboards и доступ к Grafana: [Observability](observability.md).

<a id="stop"></a>
## Остановка

Остановить production stack:

```bash
make prod-down
```

Команда останавливает production compose stack, но не удаляет volumes.

<a id="frontend"></a>
## Frontend

Frontend находится в отдельном репозитории: [psa-front](https://github.com/Winxfly/psa-front).

Frontend разворачивается отдельно. Для production важно, чтобы backend `.env` разрешал origin frontend-приложения:

```env
CORS_ALLOWED_ORIGINS=https://example.com
```

Текущий Caddyfile уже проксирует `/api/*`, `/health/live`, `/health/ready` в backend, а финальная маршрутизация frontend зависит от схемы деплоя frontend.