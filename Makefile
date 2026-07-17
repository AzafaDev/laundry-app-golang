include .env
export

.PHONY: run build migrate-up migrate-down seed-admin seed-demo docker-up docker-down dev

run:
	go run ./cmd/api
build:
	go build -o bin/server ./cmd/api
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up
migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1
seed-admin:
	go run ./cmd/seed
seed-demo:
	go run ./cmd/seed-demo
docker-up:
	docker compose up -d
docker-down:
	docker compose down
dev:
	air