.PHONY: help fmt vet lint build run test test-e2e coverage cover up down reset kafka migrate-status migrate-up migrate-down \
	nomad-agent nomad-up nomad-down nomad-reset nomad-status nomad-url test-e2e-nomad

DATABASE_URL = postgres://todo:todo@localhost:5432/todo?sslmode=disable
UNIT_PKGS = $$(go list ./... | grep -v /test/e2e)

NOMAD_DIR = deployments/nomad
# Backing services first: the todo job reads their addresses when it starts.
NOMAD_INFRA = postgres redis kafka
# Host volume folders for the Nomad jobs, the Nomad version of compose's named volumes.
NOMAD_DATA = $(CURDIR)/.nomad-data

help:
	@echo "Available targets:"
	@echo "  fmt             format code"
	@echo "  vet             vet code"
	@echo "  lint            run golangci-lint in docker"
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
	@echo "  nomad-agent     start a local Nomad dev agent (blocks, run it in its own terminal)"
	@echo "  nomad-up        deploy postgres, redis, kafka, then the todo app to Nomad"
	@echo "  nomad-down      stop and remove every Nomad job (keeps data)"
	@echo "  nomad-reset     stop every Nomad job and DELETE all postgres and kafka data"
	@echo "  nomad-status    show the status of every Nomad job"
	@echo "  nomad-url       print the address the todo app is listening on"
	@echo "  test-e2e-nomad  run e2e tests against the app running on Nomad"

fmt:
	go fmt ./...

vet: fmt
	go vet ./...

lint:
	docker compose run --rm lint

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

# Services register with the address of the network interface given here.
# The default, 127.0.0.1, would point a container back at itself, so use
# Docker's bridge: containers can reach it, other machines can't. The Nomad API
# itself stays on 127.0.0.1, since the dev agent has no authentication.
#
# The host volume folders must exist before the agent starts. Kafka runs as
# uid 1000, not as you, so its folder has to be writable by everyone. Postgres
# starts as root and takes ownership of its folder itself.
nomad-agent:
	mkdir -p "$(NOMAD_DATA)/pgdata" "$(NOMAD_DATA)/kafkadata"
	chmod 777 "$(NOMAD_DATA)/kafkadata"
	sed 's|@DATA_DIR@|$(NOMAD_DATA)|g' $(NOMAD_DIR)/agent.hcl.tpl > "$(NOMAD_DATA)/agent.hcl"
	nomad agent -dev -network-interface=docker0 -config="$(NOMAD_DATA)/agent.hcl"

# job run waits until each deployment is healthy, so the todo job starts
# only after the services it looks up are registered
nomad-up:
	nomad job run deployments/nomad/postgres.nomad.hcl
	nomad job run deployments/nomad/redis.nomad.hcl
	nomad job run deployments/nomad/kafka.nomad.hcl
	nomad job run deployments/nomad/app.nomad.hcl

nomad-down:
	-nomad job stop -purge app
	-@for job in $(NOMAD_INFRA); do nomad job stop -purge $$job; done

# The files belong to the containers' users, not to you, so delete them from
# inside a container. Keeps the folders, which the running agent expects.
nomad-reset: nomad-down
	docker run --rm -v "$(NOMAD_DATA):/data" --entrypoint sh postgres:18 \
		-c 'find /data/pgdata /data/kafkadata -mindepth 1 -delete'

nomad-status:
	nomad job status

NOMAD_URL = nomad service info -t '{{ range . }}http://{{ .Address }}:{{ .Port }}{{ "\n" }}{{ end }}' todo-app

nomad-url:
	@$(NOMAD_URL)

# The app's port on Nomad is dynamic, so look it up rather than hardcoding it.
test-e2e-nomad:
	@url="$$($(NOMAD_URL) 2>/dev/null | head -n1)"; \
	if [ -z "$$url" ]; then echo "todo-app is not registered in Nomad (run make nomad-up)"; exit 1; fi; \
	echo "testing $$url"; \
	E2E_BASE_URL="$$url" go test -count=1 ./test/e2e/

application-image-build:
	echo "to be done"