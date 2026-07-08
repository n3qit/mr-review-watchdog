.PHONY: build test vet fmt

build:
	go build -o bin/mr-review-watchdog ./cmd/mr-review-watchdog

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l .
