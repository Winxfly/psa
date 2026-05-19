# API

Примеры запросов к публичному и административному REST API.

## Оглавление

- [Base URL](#base-url)
- [Health Checks](#health-checks)
- [Authentication](#authentication)
- [Public API](#public-api)
- [Admin API](#admin-api)

<a id="base-url"></a>
## Base URL

Для local development:

```bash
API_BASE_URL=http://localhost:8080
CURL_FLAGS=
```

Для production:

```bash
API_BASE_URL=https://example.com
CURL_FLAGS=
```

Для локального smoke production stack:

```bash
API_BASE_URL=https://localhost
CURL_FLAGS=-k
```

`CURL_FLAGS=-k` нужен только для локального `https://localhost` с недоверенным сертификатом.

<a id="health-checks"></a>
## Health Checks

### Liveness

`GET /health/live`

```bash
curl $CURL_FLAGS "$API_BASE_URL/health/live"
```

Response `200 OK`:

```json
{
  "status": "ok"
}
```

### Readiness

`GET /health/ready`

```bash
curl $CURL_FLAGS "$API_BASE_URL/health/ready"
```

Response `200 OK`:

```json
{
  "status": "ok",
  "checks": {
    "db": "ok",
    "cache": "ok"
  }
}
```

Response `503 Service Unavailable`:

```json
{
  "status": "fail",
  "checks": {
    "db": "ok",
    "cache": "fail"
  }
}
```

<a id="authentication"></a>
## Authentication

### Sign in

Аутентификация пользователя и получение пары JWT-токенов.

`POST /api/v1/auth/signin`

Request body:

```json
{
  "email": "admin@example.com",
  "password": "supersecret"
}
```

```bash
curl $CURL_FLAGS -X POST "$API_BASE_URL/api/v1/auth/signin" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "supersecret"
  }'
```

Response `200 OK`:

```json
{
  "access_token": "<jwt-access-token>",
  "refresh_token": "<jwt-refresh-token>"
}
```

Для admin-запросов access token можно сохранить в переменную:

```bash
ACCESS_TOKEN="<jwt-access-token>"
```

### Refresh token

Обновление `access token` с помощью `refresh token`.

`POST /api/v1/auth/refresh`

Request body:

```json
{
  "refresh_token": "<refresh-token>"
}
```

```bash
curl $CURL_FLAGS -X POST "$API_BASE_URL/api/v1/auth/refresh" \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "<refresh-token>"
  }'
```

Response `200 OK`:

```json
{
  "access_token": "<new-access-token>",
  "refresh_token": "<new-refresh-token>"
}
```

### Logout

Инвалидация `refresh token`.

`POST /api/v1/auth/logout`

Request body:

```json
{
  "refresh_token": "<refresh-token>"
}
```

```bash
curl $CURL_FLAGS -X POST "$API_BASE_URL/api/v1/auth/logout" \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "<refresh-token>"
  }'
```

Response `200 OK`:

```json
{
  "message": "Successfully logged out"
}
```

<a id="public-api"></a>
## Public API

### Получить список активных профессий

`GET /api/v1/professions`

```bash
curl $CURL_FLAGS "$API_BASE_URL/api/v1/professions"
```

Response `200 OK`:

```json
[
  {
    "id": "6e8b30bd-8ea9-4906-89f9-00dd1c1e6653",
    "name": "Go Developer",
    "vacancy_query": "go developer OR golang"
  }
]
```

### Получить последние агрегированные данные о профессии

`GET /api/v1/professions/{id}/latest`

```bash
curl $CURL_FLAGS "$API_BASE_URL/api/v1/professions/6e8b30bd-8ea9-4906-89f9-00dd1c1e6653/latest"
```

Response `200 OK`:

```json
{
  "profession_id": "6e8b30bd-8ea9-4906-89f9-00dd1c1e6653",
  "profession_name": "Go Developer",
  "scraped_at": "2026-01-28T04:54:23Z",
  "vacancy_count": 352,
  "formal_skills": [
    {
      "skill": "golang",
      "count": 212
    }
  ],
  "extracted_skills": [
    {
      "skill": "go",
      "count": 563
    }
  ]
}
```

### Получить последние агрегированные данные о профессии и динамику вакансий за всё время

`GET /api/v1/professions/{id}/latest?trend=true`

```bash
curl $CURL_FLAGS "$API_BASE_URL/api/v1/professions/6e8b30bd-8ea9-4906-89f9-00dd1c1e6653/latest?trend=true"
```

Response `200 OK`:

```json
{
  "profession_id": "6e8b30bd-8ea9-4906-89f9-00dd1c1e6653",
  "profession_name": "Go Developer",
  "scraped_at": "2026-01-28T04:54:23Z",
  "vacancy_count": 352,
  "formal_skills": [
    {
      "skill": "golang",
      "count": 212
    }
  ],
  "extracted_skills": [
    {
      "skill": "go",
      "count": 563
    }
  ],
  "trend": [
    {
      "date": "2026-03-01T11:56:31Z",
      "vacancy_count": 330
    },
    {
      "date": "2026-03-02T00:40:15Z",
      "vacancy_count": 323
    }
  ]
}
```

### Получить динамику вакансий по профессии за всё время

`GET /api/v1/professions/{id}/trend`

```bash
curl $CURL_FLAGS "$API_BASE_URL/api/v1/professions/6e8b30bd-8ea9-4906-89f9-00dd1c1e6653/trend"
```

Response `200 OK`:

```json
{
  "profession_id": "6e8b30bd-8ea9-4906-89f9-00dd1c1e6653",
  "profession_name": "Go Developer",
  "data": [
    {
      "date": "2026-03-01T11:56:31Z",
      "vacancy_count": 330
    },
    {
      "date": "2026-03-02T00:40:15Z",
      "vacancy_count": 323
    }
  ]
}
```

<a id="admin-api"></a>
## Admin API

Все административные эндпоинты требуют `access token` с ролью `admin`.

Authorization header:

```text
Authorization: Bearer <access-token>
```

Для примеров ниже можно использовать переменную:

```bash
ACCESS_TOKEN="<jwt-access-token>"
```

### Получить список всех профессий

Возвращает активные и неактивные профессии.

`GET /api/v1/admin/professions`

```bash
curl $CURL_FLAGS "$API_BASE_URL/api/v1/admin/professions" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

Response `200 OK`:

```json
[
  {
    "id": "6e8b30bd-8ea9-4906-89f9-00dd1c1e6653",
    "name": "Go Developer",
    "vacancy_query": "go developer OR golang",
    "is_active": true
  }
]
```

Response `401 Unauthorized`:

```json
{
  "error": "Invalid token"
}
```

### Создать профессию

`POST /api/v1/admin/professions`

Request body:

```json
{
  "name": "C# Developer",
  "vacancy_query": "C#"
}
```

```bash
curl $CURL_FLAGS -X POST "$API_BASE_URL/api/v1/admin/professions" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "C# Developer",
    "vacancy_query": "C#"
  }'
```

Response `201 Created`:

```json
{
  "id": "e337f9e7-c0b6-4089-8b66-19ad3ef58ad0",
  "name": "C# Developer",
  "vacancy_query": "C#",
  "is_active": true
}
```

### Обновить профессию

`PUT /api/v1/admin/professions/{id}`

Request body:

```json
{
  "name": "C# Developer",
  "vacancy_query": "C#",
  "is_active": true
}
```

```bash
curl $CURL_FLAGS -X PUT "$API_BASE_URL/api/v1/admin/professions/e337f9e7-c0b6-4089-8b66-19ad3ef58ad0" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "C# Developer",
    "vacancy_query": "C#",
    "is_active": true
  }'
```

Response `200 OK`:

```json
{
  "id": "e337f9e7-c0b6-4089-8b66-19ad3ef58ad0",
  "name": "C# Developer",
  "vacancy_query": "C#",
  "is_active": true
}
```

### Запустить полный сбор данных

Собирает данные по всем активным профессиям, сохраняет полный результат в PostgreSQL и обновляет Redis.

`POST /api/v1/admin/scraping/archive`

```bash
curl $CURL_FLAGS -X POST "$API_BASE_URL/api/v1/admin/scraping/archive" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

Response `202 Accepted`:

```json
{
  "status": "started",
  "mode": "archive"
}
```

### Запустить оперативный сбор данных

Собирает данные по всем активным профессиям, обновляет Redis и сохраняет дневную статистику вакансий в PostgreSQL.

`POST /api/v1/admin/scraping/cache`

```bash
curl $CURL_FLAGS -X POST "$API_BASE_URL/api/v1/admin/scraping/cache" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

Response `202 Accepted`:

```json
{
  "status": "started",
  "mode": "cache"
}
```