.PHONY: build test lint clean run docker

BINARY=relay
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags="-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test -v -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)

run: build
	./$(BINARY)

docker:
	docker build -t karmagaterelay:latest .

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
