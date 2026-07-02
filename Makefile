.PHONY: build run sqlc lint test

build:
	go build ./...

run:
	go run ./cmd/worker

sqlc:
	sqlc generate

lint:
	go vet ./...

test:
	go test ./...
