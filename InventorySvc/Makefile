APP_NAME := inventory-svc
CMD_PATH := ./cmd/inventory-svc
BUILD_DIR := ./bin
SWAGGER := go run github.com/swaggo/swag/cmd/swag@v1.16.4
CONFIG ?= ./appsettings.Develop.yml

.PHONY: build run test swagger tidy clean

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) $(CMD_PATH)

run:
	go run $(CMD_PATH) -config $(CONFIG)

test:
	go test ./...

swagger:
	$(SWAGGER) init -g cmd/inventory-svc/main.go -o docs/swagger --parseDependency --parseInternal

tidy:
	go mod tidy

clean:
	if exist $(BUILD_DIR) rmdir /S /Q $(BUILD_DIR)
