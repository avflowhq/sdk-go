.PHONY: test lint fmt tidy

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

tidy:
	go mod tidy
