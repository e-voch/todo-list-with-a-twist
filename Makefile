.PHONY: help fmt vet build run test coverage cover topic kafka up down

help:
	@echo "Available targets:"
	@echo "  fmt       format code"
	@echo "  vet       vet code"
	@echo "  build     build the binary into bin/todo"
	@echo "  run       run the server"
	@echo "  test      run tests (bypasses cache)"
	@echo "  coverage  generate coverage.out"
	@echo "  cover     generate coverage and open the HTML report"
	@echo "  topic     create the kafka tasks topic (needs the broker running)"
	@echo "  kafka  show every message on the tasks topic (Ctrl+C to stop)"

fmt:
	go fmt ./...

vet: fmt
	go vet ./...

build: vet
	go build -o bin/todo ./cmd/todo

run:
	go run ./cmd/todo

test:
	go test -count=1 ./...

coverage:
	go test -coverprofile=coverage.out ./...

cover: coverage
	go tool cover -html=coverage.out

topic:
	docker compose exec -T broker /opt/kafka/bin/kafka-topics.sh \
		--bootstrap-server localhost:9092 --create --if-not-exists \
		--topic tasks --partitions 1 --replication-factor 1

kafka:
	docker compose exec broker /opt/kafka/bin/kafka-console-consumer.sh \
		--bootstrap-server localhost:9092 --topic tasks --from-beginning \
		--property print.key=true --property print.timestamp=true

up: 
	docker compose up --build --force-recreate -d

down: 
	docker compose down -v --remove-orphans

