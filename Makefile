.PHONY: help build run test coverage cover format topic

help:
	@echo "Available targets:"
	@echo "  build     build the binary into bin/todo"
	@echo "  run       run the server"
	@echo "  test      run tests (bypasses cache)"
	@echo "  coverage  generate coverage.out"
	@echo "  cover     generate coverage and open the HTML report"
	@echo "  format    format code"
	@echo "  topic     create the kafka tasks topic (needs the broker running)"

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

topic:
	docker compose exec -T broker /opt/kafka/bin/kafka-topics.sh \
		--bootstrap-server localhost:9092 --create --if-not-exists \
		--topic tasks --partitions 1 --replication-factor 1
