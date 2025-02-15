.DEFAULT_GOAL := build
.PHONY: build
build:
		@go build -ldflags="-s -w" -trimpath
.PHONY: test
test:
		@go test -v ./...
.PHONY: dry-lint
dry-lint:
		@golangci-lint run
.PHONY: lint
lint:
		@golangci-lint run --fix
