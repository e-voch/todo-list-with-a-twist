.PHONY: help fmt vet build run test coverage cover up down reset topic kafka migrate-status migrate-up migrate-down

GOOSE_ENV = GOOSE_DRIVER=postgres \
	GOOSE_DBSTRING="postgres://todo:todo@localhost:5432/todo?sslmode=disable" \
	GOOSE_MIGRATION_DIR=db/migrations

help:
	@echo "Available targets:"
	@echo "  fmt             format code"
	@echo "  vet             vet code"
	@echo "  build           build the binary into bin/todo"
	@echo "  run             run the server"
	@echo "  test            run tests (bypasses cache)"
	@echo "  coverage        generate coverage.out"
	@echo "  cover           generate coverage and open the HTML report"
	@echo "  up              start postgres, kafka and the UIs"
	@echo "  down            stop and remove the containers (keeps data)"
	@echo "  reset           stop everything and DELETE all postgres and kafka data"
	@echo "  topic           create the kafka tasks topic (needs the broker running)"
	@echo "  kafka           show every message on the tasks topic (Ctrl+C to stop)"
	@echo "  migrate-status  show which database migrations have been applied"
	@echo "  migrate-up      apply all pending database migrations"
	@echo "  migrate-down    undo the last applied database migration"

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

up:
	docker compose up -d

down:
	docker compose down --remove-orphans

reset:
	docker compose down -v --remove-orphans

topic:
	docker compose exec -T broker /opt/kafka/bin/kafka-topics.sh \
		--bootstrap-server localhost:9092 --create --if-not-exists \
		--topic tasks --partitions 1 --replication-factor 1

kafka:
	docker compose exec broker /opt/kafka/bin/kafka-console-consumer.sh \
		--bootstrap-server localhost:9092 --topic tasks --from-beginning \
		--property print.key=true --property print.timestamp=true

migrate-status:
	$(GOOSE_ENV) goose status

migrate-up:
	$(GOOSE_ENV) goose up

migrate-down:
	$(GOOSE_ENV) goose down
