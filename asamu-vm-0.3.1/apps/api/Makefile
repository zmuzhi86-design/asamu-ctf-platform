.PHONY: fmt test vet run worker migrate seed

fmt:
	gofmt -w $$(find . -name '*.go')

test:
	go test ./...

vet:
	go vet ./...

run:
	go run ./cmd/api

worker:
	go run ./cmd/worker

migrate:
	go run ./cmd/migrate up

seed:
	go run ./cmd/seed
