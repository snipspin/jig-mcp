.PHONY: build test test-race lint vet clean install cover

BINARY  := jig-mcp
MAIN    := ./cmd/jig-mcp/
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) $(MAIN)

test:
	go test ./...

test-race:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

install:
	go install $(LDFLAGS) $(MAIN)

clean:
	rm -f $(BINARY)
	rm -f testdata/echo_tool testdata/sleep_tool testdata/oom_tool
	rm -f *.out *_cover.html

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

all: vet lint test-race build
