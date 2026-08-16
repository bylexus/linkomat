SHELL := /bin/bash

GO         ?= go
NPM        ?= npm
BRUNO      ?= bru
WEB_DIR    := web
API_DIR    := api
CLI_DIR    := cli
BRUNO_DIR  := doc/bruno-api-tests
SERVER_BIN := bin/linkomat-server
CLI_BIN    := bin/linkomat

BRUNO_ENV     ?= Local
BRUNO_BASEURL ?= http://localhost:8080
BRUNO_USER    ?= alice
BRUNO_PASS    ?= hunter2

NVM_SH ?= $(HOME)/.nvm/nvm.sh

.PHONY: help build-server build-cli web-build web-embed dev clean tidy all api-test api-test-auth api-test-req api-test-reset

help:
	@echo "Targets:"
	@echo "  build-server     Build the Go server to $(SERVER_BIN)"
	@echo "                     (depends on web-embed, which builds web/dist and"
	@echo "                     copies it into api/web_dist/ for go:embed)"
	@echo "  build-cli        Build the CLI to $(CLI_BIN)"
	@echo "  web-build        Install deps and build the web PWA to web/dist"
	@echo "  web-embed        Copy web/dist into api/web_dist/ for the embedded"
	@echo "                     server binary (runs web-build first)"
	@echo "  dev              Run the web dev server and the Go server concurrently"
	@echo "  tidy             Run go mod tidy across all modules"
	@echo "  api-test         Run auth + entries + meta Bruno collections against $(BRUNO_BASEURL)"
	@echo "                     (server must already be running; override with"
	@echo "                     BRUNO_BASEURL / BRUNO_USER / BRUNO_PASS / BRUNO_ENV)"
	@echo "  api-test-auth    Run only the auth folder"
	@echo "  api-test-req     Run a single request from the collection"
	@echo "                     (set REQ=<folder>/<name>, e.g. REQ=auth/login or"
	@echo "                     REQ=entries/entries-create)"
	@echo "  api-test-reset   Reset captured token values in the Local env"
	@echo "                     (Bruno CLI 4.x writes runtime var values back"
	@echo "                     into Local.bru after each run; run this before"
	@echo "                     committing to keep tokens out of git)"
	@echo "  clean            Remove build artifacts"

NVM_LOAD = if [ -s "$(NVM_SH)" ]; then . "$(NVM_SH)" && nvm use > /dev/null; fi

BRUNO_VARS = --env-var baseUrl=$(BRUNO_BASEURL) --env-var username=$(BRUNO_USER) --env-var password=$(BRUNO_PASS)

build-server: web-embed
	@mkdir -p bin
	$(GO) build -o $(SERVER_BIN) ./$(API_DIR)

build-cli:
	@mkdir -p bin
	$(GO) build -o $(CLI_BIN) ./$(CLI_DIR)

web-build:
	cd $(WEB_DIR) && bash -lc '$(NVM_LOAD) && $(NPM) install && $(NPM) run build'

web-embed: web-build
	@rm -rf $(API_DIR)/web_dist
	@mkdir -p $(API_DIR)/web_dist
	@cp -R $(WEB_DIR)/dist/. $(API_DIR)/web_dist/
	@echo "Embedded $$(ls $(API_DIR)/web_dist | wc -l | tr -d ' ') top-level files into api/web_dist/"

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

all: build-server build-cli

api-test:
	cd $(BRUNO_DIR) && $(BRUNO) run auth entries meta --env $(BRUNO_ENV) $(BRUNO_VARS) || true

api-test-auth:
	cd $(BRUNO_DIR) && $(BRUNO) run auth --env $(BRUNO_ENV) $(BRUNO_VARS)

api-test-req:
	@if [ -z "$(REQ)" ]; then echo "Usage: make api-test-req REQ=<request-name>"; exit 2; fi
	cd $(BRUNO_DIR) && $(BRUNO) run $(REQ).bru --env $(BRUNO_ENV) $(BRUNO_VARS)

api-test-reset:
	@printf 'vars {\n  baseUrl: http://localhost:8080\n  username: alice\n  password: hunter2\n  badPassword: definitely-wrong\n  deactivatableUsername: charlie\n  bogusRefreshToken: deadbeef0000000000000000000000000000000000000000000000000000beef\n  entryId: 01923456-7890-7abc-9def-0123456789ab\n  lastAccessToken: \n  lastRefreshToken: \n}\n\nvars:secret [\n  lastAccessToken,\n  lastRefreshToken\n]\n' > $(BRUNO_DIR)/environments/Local.bru

clean:
	rm -rf bin
	rm -rf $(WEB_DIR)/dist
	rm -rf $(WEB_DIR)/node_modules
	rm -rf $(API_DIR)/web_dist
