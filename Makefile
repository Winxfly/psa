# Запуск миграций, но должен быть поднят postgres
migrate-up:
	docker compose run --rm \
    		-e CONFIG_PATH=/config/local.yaml \
    		-v ${PWD}/config:/config \
    		-v ${PWD}/migrations:/migrations \
    		-v ${PWD}/.env:/app/.env \
    		migrator --migrations-path=/migrations --up

# Удалить все миграции (полностью), но должен быть поднят postgres
migrate-down:
	docker compose run --rm \
    		-e CONFIG_PATH=/config/local.yaml \
    		-v ${PWD}/config:/config \
    		-v ${PWD}/migrations:/migrations \
    		-v ${PWD}/.env:/app/.env \
    		migrator --migrations-path=/migrations --down

# Принудительно выставить версию миграции в 0 (для сброса dirty-состояния)
migrate-force-0:
	docker compose run --rm \
		-e CONFIG_PATH=/config/local.yaml \
		-v ${PWD}/config:/config \
		-v ${PWD}/migrations:/migrations \
		-v ${PWD}/.env:/app/.env \
		migrator --migrations-path=/migrations --force=0

# Принудительно выставить версию миграции в 1
migrate-force-1:
	docker compose run --rm \
		-e CONFIG_PATH=/config/local.yaml \
		-v ${PWD}/config:/config \
		-v ${PWD}/migrations:/migrations \
		-v ${PWD}/.env:/app/.env \
		migrator --migrations-path=/migrations --force=1

# Генерация кода для БД
sqlc-generate:
	docker build -t psa-sqlc -f tools/sqlc/Dockerfile .
	docker run --rm \
		-v $(PWD):/app \
		-w /app \
		psa-sqlc generate

build-create-admin:
	docker build -t create-admin -f tools/admin/Dockerfile .

# Создать админа. Пример: make create-admin ADMIN_EMAIL=admin@example.com ADMIN_PASSWORD=supersecret
create-admin: build-create-admin
	docker compose run --rm \
		-e CONFIG_PATH=/config/local.yaml \
		-e ADMIN_EMAIL=$(ADMIN_EMAIL) \
		-e ADMIN_PASSWORD=$(ADMIN_PASSWORD) \
		-v ${PWD}/config:/config \
		create-admin

# Поднять все контейнеры
up:
	docker compose up --build app

# Остановить все контейнеры
down:
	docker compose down

# Поднять только PostgreSQL
postgres-up:
	docker compose up -d postgres

# Остановить только PostgreSQL
postgres-down:
	docker compose stop postgres

# Полностью удалить контейнер PostgreSQL и том
postgres-clean:
	docker compose down -v --remove-orphans