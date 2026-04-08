.DEFAULT_GOAL := help

COMPOSE ?= docker compose
TAIL ?= 120

.PHONY: help launch up build down stop restart reset ps logs webapp-logs nginx-logs postgres-logs pgadmin-logs

help:
	@echo "Available targets:"
	@echo "  make launch         Build and start all services in detached mode"
	@echo "  make up             Same as launch"
	@echo "  make build          Build the service images without starting them"
	@echo "  make down           Stop and remove containers"
	@echo "  make stop           Stop running containers"
	@echo "  make restart        Restart running containers"
	@echo "  make reset          Remove volumes, then rebuild and start everything"
	@echo "  make ps             Show service status"
	@echo "  make logs           Follow all service logs"
	@echo "  make webapp-logs    Follow webapp logs"
	@echo "  make nginx-logs     Follow nginx logs"
	@echo "  make postgres-logs  Follow postgres logs"
	@echo "  make pgadmin-logs   Follow pgadmin logs"

launch: up

up:
	$(COMPOSE) up --build -d

build:
	$(COMPOSE) build

down:
	$(COMPOSE) down

stop:
	$(COMPOSE) stop

restart:
	$(COMPOSE) restart

reset:
	$(COMPOSE) down -v --remove-orphans
	$(COMPOSE) up --build -d

ps:
	$(COMPOSE) ps

logs:
	$(COMPOSE) logs -f --tail=$(TAIL)

webapp-logs:
	$(COMPOSE) logs -f --tail=$(TAIL) webapp

nginx-logs:
	$(COMPOSE) logs -f --tail=$(TAIL) nginx

postgres-logs:
	$(COMPOSE) logs -f --tail=$(TAIL) postgres

pgadmin-logs:
	$(COMPOSE) logs -f --tail=$(TAIL) pgadmin
