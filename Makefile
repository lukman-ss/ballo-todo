.PHONY: run build test race vet fmt check clean help

APP_NAME := server
BIN_DIR := bin
MAIN_SRC := ./cmd/server

run:
	go run $(MAIN_SRC)

build:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(APP_NAME) $(MAIN_SRC)

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files found:" && gofmt -l . && exit 1)

check: fmt vet test race

clean:
	rm -rf $(BIN_DIR) coverage.out
