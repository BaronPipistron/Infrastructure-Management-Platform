APP_NAME := parser-svc
CMD_PATH := ./cmd/parser-svc
BUILD_DIR := ./bin
SWAGGER := go run github.com/swaggo/swag/cmd/swag@v1.16.4
GOLANGCI := go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
CONFIG ?= ./configs/appconfig.Develop.yml

.PHONY: build run test lint swagger tidy clean

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) $(CMD_PATH)

run:
	go run $(CMD_PATH) -config $(CONFIG)

test:
	go test ./...

lint:
	$(GOLANGCI) run --config .golangci.yml ./cmd/... ./internal/...

swagger:
	$(SWAGGER) init -g cmd/parser-svc/main.go -o docs/swagger --parseDependency --parseInternal

tidy:
	go mod tidy

clean:
	if exist $(BUILD_DIR) rmdir /S /Q $(BUILD_DIR)
