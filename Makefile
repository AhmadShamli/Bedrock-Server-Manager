.PHONY: all build build-web build-server clean test run install update check release

BIN_DIR := bin
BINARY := $(BIN_DIR)/bedrock-server-manager

all: build

build-web:
	@cd web && (test -d node_modules || npm install) && npm run build

build-server:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY) ./cmd/manager

build:
	@bash ./scripts/build.sh

test:
	go test -v ./...

run: build
	./$(BINARY)

check:
	@bash ./install.sh --check

install:
	@bash ./install.sh

update:
	@bash ./install.sh --upgrade

release:
	@bash ./scripts/build_release.sh

clean:
	rm -rf $(BIN_DIR) web/dist dist
