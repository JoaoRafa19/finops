APP_NAME=finops
CMD_PATH=./cmd/finops/

.PHONY: run build test fmt tidy start_db startf migrate gen sqlc

run:
	go run $(CMD_PATH)
	
build:
	go build -o bin/$(APP_NAME) $(CMD_PATH)

test:
	go test ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy

start_db:
	docker compose -f ./deploy/docker/docker-compose.yaml  up postgres -d

startf:
	templ generate --watch

migrate:
	set -a; . ./.env; set +a; tern migrate \
    -c ./internal/store/migrations/tern.conf \
    --migrations ./internal/store/migrations/

sqlc:
	@if command -v sqlc >/dev/null 2>&1; then \
		sqlc generate -f ./internal/store/sqlc.yaml; \
	elif [ -x "$$(go env GOPATH)/bin/sqlc" ]; then \
		"$$(go env GOPATH)/bin/sqlc" generate -f ./internal/store/sqlc.yaml; \
	else \
		echo "sqlc binary not found; running via go run..."; \
		go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate -f ./internal/store/sqlc.yaml; \
	fi

gen:
	$(MAKE) sqlc

setup:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/jackc/tern/v2@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
