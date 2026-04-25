DOCKER_COMPOSE := docker compose --project-directory . --env-file .env -f infra/docker-compose.yaml
DOCKER_COMPOSE_PROD := docker compose --project-directory . --env-file .env -f infra/docker-compose.prod.yaml

# Основные dev-команды

# Поднять backend, db, cache
up:
	$(DOCKER_COMPOSE) up --build --remove-orphans backend

# Поднять весь стек
full-up:
	$(DOCKER_COMPOSE) up --build --remove-orphans backend prometheus grafana loki alloy

# Поднять production-стек
prod-up:
	$(DOCKER_COMPOSE_PROD) up --build -d --remove-orphans caddy backend prometheus grafana loki alloy

# Остановить все контейнеры
down:
	$(DOCKER_COMPOSE) down

# Остановить production-стек
prod-down:
	$(DOCKER_COMPOSE_PROD) down

# Поднять observability стек
obs-up:
	$(DOCKER_COMPOSE) up -d prometheus grafana loki alloy

# Остановить observability стек
obs-down:
	$(DOCKER_COMPOSE) stop prometheus grafana loki alloy

# Управление сервисами

# Поднять postgres
postgres-up:
	$(DOCKER_COMPOSE) up -d postgres

# Остановить postgres
postgres-down:
	$(DOCKER_COMPOSE) stop postgres

# Поднять redis
redis-up:
	$(DOCKER_COMPOSE) up -d redis

# Остановить redis
redis-down:
	$(DOCKER_COMPOSE) stop redis

# Поднять prometheus
prometheus-up:
	$(DOCKER_COMPOSE) up -d prometheus

# Остановить prometheus
prometheus-down:
	$(DOCKER_COMPOSE) stop prometheus

# Поднять loki
loki-up:
	$(DOCKER_COMPOSE) up -d loki

# Остановить loki
loki-down:
	$(DOCKER_COMPOSE) stop loki

# Поднять alloy
alloy-up:
	$(DOCKER_COMPOSE) up -d alloy

# Остановить alloy
alloy-down:
	$(DOCKER_COMPOSE) stop alloy

# Поднять grafana
grafana-up:
	$(DOCKER_COMPOSE) up -d grafana

# Остановить grafana
grafana-down:
	$(DOCKER_COMPOSE) stop grafana

# Миграции и админка

# Запуск миграций, но должен быть поднят postgres
migrate-up:
	$(DOCKER_COMPOSE) run --rm migrator --migrations-path=/migrations --up

# Запуск миграций в production-стеке, но должен быть поднят postgres
prod-migrate-up:
	$(DOCKER_COMPOSE_PROD) run --rm migrator --migrations-path=/migrations --up

# Откатить все миграции, должен быть поднят postgres
migrate-down:
	$(DOCKER_COMPOSE) run --rm migrator --migrations-path=/migrations --down

# Принудительно выставить версию миграции в 0 (для сброса dirty-состояния)
migrate-force-0:
	$(DOCKER_COMPOSE) run --rm migrator --migrations-path=/migrations --force=0

# Принудительно выставить версию миграции в 1
migrate-force-1:
	$(DOCKER_COMPOSE) run --rm migrator --migrations-path=/migrations --force=1

build-create-admin:
	docker build -t create-admin -f tools/admin/Dockerfile .

# Создать админа. Пример: make create-admin ADMIN_EMAIL=admin@example.com ADMIN_PASSWORD=supersecret
create-admin: build-create-admin
	$(DOCKER_COMPOSE) run --rm \
		-e ADMIN_EMAIL=$(ADMIN_EMAIL) \
		-e ADMIN_PASSWORD=$(ADMIN_PASSWORD) \
		create-admin

# Создать админа в production-стеке
prod-create-admin: build-create-admin
	$(DOCKER_COMPOSE_PROD) run --rm \
		-e ADMIN_EMAIL=$(ADMIN_EMAIL) \
		-e ADMIN_PASSWORD=$(ADMIN_PASSWORD) \
		create-admin

# Генерация кода для работы с БД
sqlc-generate:
	docker build -t psa-sqlc -f tools/sqlc/Dockerfile .
	docker run --rm \
		-v $(PWD):/app \
		-w /app \
		psa-sqlc generate

# Качество

fmt:
	go fmt ./...

test:
	go test ./...

test-integration:
	go test -tags=integration -v -race ./...

test-integration-auth:
	go test -tags=integration -v -race ./internal/service/auth/...

test-integration-repo:
	go test -tags=integration -v -race ./internal/repository/postgresql/...

test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

GOLANGCI_LINT_VERSION=v2.11.3

# Запуск линтера
lint:
	docker run --rm \
		-v $(PWD):/app \
		-w /app \
		golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) \
		golangci-lint run

# Запуск линтера и автоматические исправления
lint-fix:
	docker run --rm \
		-v $(PWD):/app \
		-w /app \
		golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) \
		golangci-lint run --fix

# Проверка форматирования
lint-fmt:
	docker run --rm \
		-v $(PWD):/app \
		-w /app \
		golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) \
		golangci-lint fmt

# Генерация моков (все пакеты из .mockery.yaml)
mocks-generate:
	docker run --rm \
		-v $(PWD):/app \
		-w /app \
		vektra/mockery:v3.7.0 \
		--config=.mockery.yaml
