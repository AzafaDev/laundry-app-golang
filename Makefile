include .env
export

.PHONY: run build migrate-up migrate-down docker-up docker-down

run:
	go run ./cmd/api
build:
	go build -o bin/server ./cmd/api
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up
migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1
docker-up:
	docker compose up -d
docker-down:
	docker compose down