.PHONY: build install clean test

BINARY_NAME := ralph
BUILD_DIR := bin
MAIN_PKG := ./cmd/ralph

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) ~/go/bin/

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./...
