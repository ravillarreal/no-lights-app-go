.PHONY: build test vet fmt run-api run-bot lint load-test

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	go vet ./...
	gofmt -l .

run-api:
	go run ./cmd/api

run-bot:
	go run ./cmd/bot

load-test:
	k6 run k6/load.js -e BASE_URL=http://localhost:8000
