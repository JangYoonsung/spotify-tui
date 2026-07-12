.PHONY: build run test fmt vet lint

build:
	go build -o bin/spotify-tui ./cmd/spotify-tui

run: build
	./bin/spotify-tui

test:
	go test ./...

fmt:
	gofmt -l .

vet:
	go vet ./...

lint:
	golangci-lint run ./...
