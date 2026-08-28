APP := hidbootloader-cli
VERSION ?= dev
GO ?= go

.PHONY: all build static test clean

all: build

build:
	mkdir -p bin
	$(GO) build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o bin/$(APP) ./cmd/$(APP)

static:
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o bin/$(APP) ./cmd/$(APP)

test:
	$(GO) test ./...

clean:
	rm -rf bin

