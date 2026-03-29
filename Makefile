# Go toolchain (override if needed): make GO=/home/linuxbrew/.linuxbrew/bin/go build
GO ?= go

.PHONY: build test run vet fmt all help

help:
	@echo "Targets:"
	@echo "  make build  - build binary to ./bin/gocrawl"
	@echo "  make test   - go test ./..."
	@echo "  make run    - go run ./cmd"
	@echo "  make vet    - go vet ./..."
	@echo "  make fmt    - go fmt ./..."
	@echo "  make all    - fmt vet test build"

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

build:
	@mkdir -p bin
	$(GO) build -o bin/gocrawl ./cmd

run:
	$(GO) run ./cmd

all: fmt vet test build
