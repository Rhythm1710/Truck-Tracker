.PHONY: help run faketracker register-device build sqlc migrate-up migrate-down tidy test docker

DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/trucktracker?sslmode=disable

help:
	@echo "Targets:"
	@echo "  run            run the server (uses .env)"
	@echo "  faketracker    simulate a Pictor PS10A device for E2E testing"
	@echo "  register-device  add a user+device  (PHONE=+91... IMEI=... PLATE='HR 55 AB 1234')"
	@echo "  build          build static binary into ./bin/server"
	@echo "  sqlc           regenerate sqlc Go from queries.sql"
	@echo "  migrate-up     apply pending migrations"
	@echo "  migrate-down   rollback last migration"
	@echo "  tidy           go mod tidy"
	@echo "  test           run unit tests"
	@echo "  docker         build container image"

run:
	@if [ ! -f .env ]; then echo "Missing .env (copy .env.example)"; exit 1; fi
	@set -a && . ./.env && set +a && go run ./cmd/server

faketracker:  ## Run the fake Pictor PS10A device against a local server
	go run ./cmd/faketracker -addr 127.0.0.1:5023 -imei 123456789012345 -lat 29.6857 -lng 76.9905 -interval 5s

register-device:  ## Register a user+device. Required: PHONE, IMEI, PLATE. Optional: NAME, LABEL.
	@if [ -z "$(PHONE)" ] || [ -z "$(IMEI)" ] || [ -z "$(PLATE)" ]; then \
		echo "Usage: make register-device PHONE=+919876543210 IMEI=123456789012345 PLATE='HR 55 AB 1234' [NAME=Papa] [LABEL='Karnal route']"; \
		exit 1; \
	fi
	@if [ ! -f .env ]; then echo "Missing .env (copy .env.example)"; exit 1; fi
	@set -a && . ./.env && set +a && go run ./cmd/registerdevice \
		-phone "$(PHONE)" -imei "$(IMEI)" -plate "$(PLATE)" \
		-name "$(NAME)" -label "$(LABEL)"

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/server -ldflags="-s -w" ./cmd/server

sqlc:
	sqlc generate

migrate-up:
	migrate -database "$(DATABASE_URL)" -path migrations up

migrate-down:
	migrate -database "$(DATABASE_URL)" -path migrations down 1

tidy:
	go mod tidy

test:
	go test ./...

docker:
	docker build -t truck-tracker-backend:latest .
