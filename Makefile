COMPOSE = docker compose --env-file .env -f deployments/docker-compose.yml

.PHONY: up down logs test test-integration frontend-test build key

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f --tail=100

test:
	cd apps/backend && go test ./...
	cd apps/frontend && npm test

test-integration:
	docker compose -f deployments/docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from tester
	docker compose -f deployments/docker-compose.test.yml down

frontend-test:
	cd apps/frontend && npm test

build:
	$(COMPOSE) build

key:
	openssl rand -base64 32
