GO := go
BINARY := bin/ricket

.PHONY: build test lint clean release

build:
	$(GO) build -o $(BINARY) ./cmd/ricket

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...

clean:
	rm -rf bin/ .ricket/

release:
	goreleaser release --clean
