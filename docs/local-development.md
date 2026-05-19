# Local Development

Локальный запуск backend-сервиса и инфраструктуры для разработки.

## Оглавление

- [Требования](#requirements)
- [Подготовка](#setup)
- [Backend, PostgreSQL и Redis](#backend-stack)
- [Миграции и администратор](#migrations-admin)
- [Полный local stack](#full-stack)
- [Observability](#observability)
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

Для local development обычно используются такие значения:

```env
CONFIG_PATH=/config/local.yaml
APP_HOST_PORT=8080
DB_HOST_PORT=5433
REDIS_HOST_PORT=6379
PROMETHEUS_HOST_PORT=9090
GRAFANA_HOST_PORT=3001
LOKI_HOST_PORT=3100
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080
```

Если не указать ключи HH API, backend сможет стартовать, но сбор данных с hh.ru будет недоступен.

<a id="backend-stack"></a>
## Backend, PostgreSQL и Redis

Поднять backend и его зависимости:

```bash
make up
```

Команда поднимает:

- `backend`
- `postgres`
- `redis`

Локальный API:

`http://localhost:8080`

Проверка:

```bash
curl http://localhost:8080/health/ready
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

<a id="migrations-admin"></a>
## Миграции и администратор

Если БД ещё не инициализирована, применить миграции:

```bash
make migrate-up
```

Создать администратора:

```bash
make create-admin ADMIN_EMAIL=admin@example.com ADMIN_PASSWORD=supersecret
```

Администратор нужен для закрытых API: управление профессиями и ручной запуск scraping. Наличие администратора необязательно.

<a id="full-stack"></a>
## Полный local stack

Поднять backend, PostgreSQL, Redis и observability:

```bash
make full-up
```

По умолчанию будут доступны:

- API: `http://localhost:8080`
- PostgreSQL: `localhost:5433`
- Redis: `localhost:6379`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3001`
- Loki: `http://localhost:3100`

Если host-порты изменены в `.env`, адреса тоже изменятся.

<a id="observability"></a>
## Observability

Если backend уже запущен и нужно отдельно поднять observability stack:

```bash
make obs-up
```

Остановить только observability stack:

```bash
make obs-down
```

Подробнее про dashboards и доступ к Grafana: [Observability](observability.md).

<a id="stop"></a>
## Остановка

Остановить local stack:

```bash
make down
```

Команда останавливает local compose stack, но не удаляет volumes.

<a id="frontend"></a>
## Frontend

Frontend находится в отдельном репозитории: [psa-front](https://github.com/Winxfly/psa-front).

Для local development frontend обычно запускается отдельно и ходит в backend через `/api/*`.

Если frontend запущен на `http://localhost:3000`, этот origin должен быть разрешён в backend `.env`:

```env
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080
```