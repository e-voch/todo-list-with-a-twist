.PHONY: help fmt vet build run test test-e2e coverage cover up down reset kafka migrate-status migrate-up migrate-down

DATABASE_URL = postgres://todo:todo@localhost:5432/todo?sslmode=disable
UNIT_PKGS = $$(go list ./... | grep -v /test/e2e)

help:
	@echo "Available targets:"
	@echo "  fmt             format code"
	@echo "  vet             vet code"
	@echo "  build           build the binary into bin/todo"
	@echo "  run             run the server"
	@echo "  test            run unit and integration tests, skips e2e (bypasses cache)"
	@echo "  test-e2e        run e2e tests against the running app (make up && make run first)"
	@echo "  coverage        generate coverage.out (skips e2e)"
	@echo "  cover           generate coverage and open the HTML report"
	@echo "  up              start the stack, apply migrations and create the kafka topic"
	@echo "  down            stop and remove the containers (keeps data)"
	@echo "  reset           stop everything and DELETE all postgres and kafka data"
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
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/todo

test:
	go test -race -count=1 $(UNIT_PKGS)

test-e2e:
	go test -count=1 ./test/e2e/

coverage:
	go test -coverprofile=coverage.out $(UNIT_PKGS)

cover: coverage
	go tool cover -html=coverage.out

up:
	docker compose up -d --build

down:
	docker compose down --remove-orphans

reset:
	docker compose down -v --remove-orphans

kafka:
	docker compose exec broker /opt/kafka/bin/kafka-console-consumer.sh \
		--bootstrap-server localhost:9092 --topic tasks --from-beginning \
		--property print.key=true --property print.timestamp=true

migrate-status:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate status

migrate-up:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate up

migrate-down:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate down

application-image-build:
	echo "to be done"