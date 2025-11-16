# Makefile for Docker Compose operations

.PHONY: rebuild
rebuild:
	docker-compose down && docker-compose build --no-cache

.PHONY: rebuild-up
rebuild-up:
	docker-compose down && docker-compose build && docker-compose up -d && docker-compose logs -f

.PHONY: down
down:
	docker-compose down

.PHONY: up
up:
	docker-compose up -d

.PHONY: logs
logs:
	docker-compose logs -f

.PHONY: validate
validate:
	go fmt ./...
	go vet ./...
	go test ./...
	@git grep "pkg/bot" pkg/fsm pkg/state || true

.DEFAULT_GOAL := rebuild-up
