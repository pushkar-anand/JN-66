-include .env
export

.PHONY: generate mocks test fmt migrate-up migrate-down seed build run eval

generate:
	go tool sqlc generate

mocks:
	go generate ./internal/tools/...

test:
	go test ./...

fmt:
	gofmt -w .
	go tool goreturns -w .

migrate-up:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
		-database "$(DATABASE_URL)" -path internal/db/migrations up

migrate-down:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
		-database "$(DATABASE_URL)" -path internal/db/migrations down 1

seed:
	docker compose exec -T postgres psql -U finagent -d finagent -f - < scripts/seed.sql

build:
	go build -o bin/finagent ./cmd/finagent

run:
	go run ./cmd/finagent

eval:
	go run ./cmd/eval/... --config config/config.yaml
