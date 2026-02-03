## PSA - Professional Skills Analyzer

Сервис для анализа востребованных профессиональных навыков на основе данных hh.ru API.

*Проект на стадии MVP и предназначен для демонстрации*

### О проекте

PSA автоматически собирает вакансии по заданным профессиям, извлекает требования к соискателям и агрегирует
статистику по навыкам.

Источником данных является hh.ru API.
Результаты доступны через REST API.

### Основные возможности

- Интеграция с hh.ru API (OAuth 2.0)
- Извлечение навыков: формальные навыки из поля "ключевые навыки" и неявные навыки из описания вакансии с помощью
алгоритма [n-gram](https://en.wikipedia.org/wiki/N-gram)
- Агрегация навыков по частоте упоминаний
- Аутентификация и авторизация JWT
- Публичные и административные API
- Автоматическое обновление данных по расписанию

### Стек

- **Backend**: Go, стандартный net/http, slog

- **БД**: PostgreSQL (pgx)

- **Кэш**: Redis

- **Миграции**: [golang-migrate](github.com/golang-migrate/migrate)

- **Генерация SQL**: [sqlc](https://github.com/sqlc-dev/sqlc)

- **Конфигурация**: [cleanenv](github.com/ilyakaznacheev/cleanenv)

- **Контейнеризация**: Docker & Docker Compose

- **Background jobs**: [gocron](https://github.com/go-co-op/gocron)

- **Rate limiting для hh.ru API**: rate.Limiter (5 rps - требование hh.ru API)

- **Retry strategy**: Equal Jitter

### Как это работает

- Для каждой профессии формируется поисковый запрос
- Загружаются вакансии с hh.ru API
- Извлекаются: формальные навыки, навыки из текста вакансий
- Навыки агрегируются по частоте
- Данные сохраняются в кэш или кэш и БД в зависимости от расписания
- Результаты доступны через REST API

### Запуск

Для запуска необходимы: docker и docker compose, make

Зарегистрировать приложение в [HeadHunter API](https://dev.hh.ru/) (необязательно для запуска, но сбор данных будет недоступен)

Клонировать репозиторий:

```bash
git clone https://github.com/Winxfly/psa.git
```

Перейти в директорию проекта:

```bash
cd psa
```

Создать файл .env по примеру .env.example:

```bash
cp .env.example .env
```
> **Для запуска и проверки работоспособности этого достаточно. Если не указать ключи для hh API, то сбор данных будет недоступен.**

Запуск PostgreSQL: `make postgres-up`

Применение миграций БД: `make migrate-up`

Запуск приложения: `make up`

## API

All examples use curl.  
GET requests are shown without `-X GET` for brevity.

### Roles

- **admin** — access to administrative API

#### Создать админа (по необходимости)

```bash
make create-admin ADMIN_EMAIL=admin@example.com ADMIN_PASSWORD=supersecret
```

#### Health check

```bash
curl http://localhost:8080/health
```

Response 200 OK:
```
ok
```

#### API availability check
Проверка API V1

```bash
curl http://localhost:8080/api/v1/health
```

Response 200 OK:
```
ok
```

### Аутентификация

#### Sign in

Аутентификация пользователя и получение пары JWT-токенов

##### POST /api/v1/auth/signin

Request body:
```json
{
  "email": "admin@example.com",
  "password": "supersecret"
}
```

```bash
curl -X POST http://localhost:8080/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "supersecret"
  }'
```

Response 200 OK:
```json
{
  "access_token": "<jwt-access-token>",
  "refresh_token": "<jwt-refresh-token>"
}
```

#### Refresh token

Обновление access token с помощью refresh token

##### POST /api/v1/auth/refresh

Request body:
```json
{
  "refresh_token": "<refresh-token>"
}
```

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "<refresh-token>"
  }'
```

Response 200 OK:
```json
{
  "access_token": "<new-access-token>",
  "refresh_token": "<new-refresh-token>"
}
```

#### Logout

Инвалидация refresh token

##### POST /api/v1/auth/logout

Request body:
```json
{
  "refresh_token": "<refresh-token>"
}
```

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "<refresh-token>"
  }'
```

Response 200 OK:
```json
{
  "message": "Successfully logged out"
}
```

### Публичные API

#### Получить список активных профессий

##### GET /api/v1/professions

```bash
curl http://localhost:8080/api/v1/professions
```
Response 200 OK:
```json
[
  {
    "id": "6e8b30bd-8ea9-4906-89f9-00dd1c1e6653",
    "name": "Go Developer",
    "vacancy_query": "go developer OR golang"
  }
]
```

#### Получить последние агрегированные данные по профессии

##### GET /api/v1/professions/{id}/latest

```bash
curl http://localhost:8080/api/v1/professions/6e8b30bd-8ea9-4906-89f9-00dd1c1e6653/latest
```

Response 200 OK:
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
### Административные API
Все административные эндпоинты требуют access token с ролью admin

#### Авторизация

Заголовок:
```
Authorization: Bearer <access-token>
```

#### Получить список всех профессий

Возвращает активные и неактивные профессии, используемые для фоновой обработки

##### GET /api/v1/admin/professions

```bash
curl http://localhost:8080/api/v1/admin/professions \
  -H "Authorization: Bearer <access-token>"
```

Response 200 OK:
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
Ошибка авторизации (невалидный токен)
```bash
curl http://localhost:8080/api/v1/admin/professions \
  -H "Authorization: Bearer invalid-token"
```

Response 401 Unauthorized:
```json
{
  "error": "Invalid token"
}
```

#### Создать профессию

##### POST /api/v1/admin/professions

Request body:
```json
{
  "name": "C# Developer",
  "vacancy_query": "C#"
}
```

```bash
curl -X POST http://localhost:8080/api/v1/admin/professions \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "C# Developer",
    "vacancy_query": "C#"
  }'
```

Response 201 Created:
```json
{
  "id": "e337f9e7-c0b6-4089-8b66-19ad3ef58ad0",
  "name": "C# Developer",
  "vacancy_query": "C#",
  "is_active": true
}
```

#### Обновить профессию

##### PUT /api/v1/admin/professions/{id}

Request body:
```json
{
  "name": "C# Developer",
  "vacancy_query": "C#",
  "is_active": true
}

```

```bash
curl -X PUT http://localhost:8080/api/v1/admin/professions/e337f9e7-c0b6-4089-8b66-19ad3ef58ad0 \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "C# Developer",
    "vacancy_query": "C#",
    "is_active": true
  }'
```

Response 200 OK:
```json
{
  "id": "e337f9e7-c0b6-4089-8b66-19ad3ef58ad0",
  "name": "C# Developer",
  "vacancy_query": "C#",
  "is_active": true
}
```