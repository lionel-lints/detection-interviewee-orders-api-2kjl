.PHONY: run test test-race load load-race tidy

run:
	go run ./cmd/orders-api

test:
	go test ./...

test-race:
	go test -race ./...

load:
	go run ./cmd/loadgen

load-race:
	go run -race ./cmd/loadgen

tidy:
	go mod tidy
