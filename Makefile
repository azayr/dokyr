.PHONY: dev api web build check

dev:
	docker compose up --build

api:
	go run ./cmd/server

web:
	cd web && pnpm dev

build:
	cd web && pnpm build
	go build ./cmd/server

check:
	go test ./cmd/... ./internal/...
	cd web && pnpm check
