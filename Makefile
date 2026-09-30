.PHONY: help build run test coverage cover format

help:
	@echo "Available targets:"
	@echo "  build     build the binary into bin/todo"
	@echo "  run       run the server"
	@echo "  test      run tests (bypasses cache)"
	@echo "  coverage  generate coverage.out"
	@echo "  cover     generate coverage and open the HTML report"
	@echo "  format    format code"

build:
	go build -o bin/todo ./cmd/todo

run:
	go run ./cmd/todo

test:
	go test -count=1 ./...

coverage:
	go test -coverprofile=coverage.out ./...

cover: coverage
	go tool cover -html=coverage.out

format:
	go fmt ./...
