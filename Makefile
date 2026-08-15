SHELL := /bin/bash

GO         ?= go
NPM        ?= npm
WEB_DIR    := web
API_DIR    := api
CLI_DIR    := cli
SERVER_BIN := bin/linkomat-server
CLI_BIN    := bin/linkomat

NVM_SH ?= $(HOME)/.nvm/nvm.sh

.PHONY: help build-server build-cli web-build dev clean tidy all

help:
	@echo "Targets:"
	@echo "  build-server  Build the Go server to $(SERVER_BIN)"
	@echo "  build-cli     Build the CLI to $(CLI_BIN)"
	@echo "  web-build     Install deps and build the web PWA"
	@echo "  dev           Run the web dev server and the Go server concurrently"
	@echo "  tidy          Run go mod tidy across all modules"
	@echo "  clean         Remove build artifacts"

NVM_LOAD = if [ -s "$(NVM_SH)" ]; then . "$(NVM_SH)" && nvm use > /dev/null; fi

build-server:
	@mkdir -p bin
	$(GO) build -o $(SERVER_BIN) ./$(API_DIR)

build-cli:
	@mkdir -p bin
	$(GO) build -o $(CLI_BIN) ./$(CLI_DIR)

web-build:
	cd $(WEB_DIR) && bash -lc '$(NVM_LOAD) && $(NPM) install && $(NPM) run build'

dev:
	@echo "Starting web dev server on http://localhost:5173"
	@echo "Starting Go server on :8080"
	@trap 'kill 0' EXIT INT TERM; \
	bash -lc 'cd $(WEB_DIR) && $(NVM_LOAD) && $(NPM) run dev' & \
	$(GO) run ./$(API_DIR) & \
	wait

tidy:
	cd $(API_DIR) && $(GO) mod tidy
	cd $(CLI_DIR) && $(GO) mod tidy
	cd shared && $(GO) mod tidy

all: build-server build-cli web-build

clean:
	rm -rf bin
	rm -rf $(WEB_DIR)/dist
	rm -rf $(WEB_DIR)/node_modules
