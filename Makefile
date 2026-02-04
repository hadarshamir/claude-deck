.PHONY: build run clean install test

BINARY_NAME=claude-deck
BUILD_DIR=./bin
CMD_DIR=./cmd/claude-deck

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

clean:
	rm -rf $(BUILD_DIR)
	go clean

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) ~/bin/

test:
	go test -v ./...

# Development helpers
fmt:
	go fmt ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

# Build for release
release:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
