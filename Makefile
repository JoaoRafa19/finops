APP_NAME=finops
CMD_PATH=./cmd/finops/
include .env
export 

.PHONY: run build test fmt tidy start_db startf migrate gen sqlc

run: # Run the application
	go run $(CMD_PATH)
	
build: # Build the application
	go build -o bin/$(APP_NAME) $(CMD_PATH)

test: # Run all tests
	go test ./...

fmt: # Format the code
	go fmt ./...

tidy: # Tidy up the go modules
	go mod tidy

start_db: # Start the postgres and redis with docker compose
	docker compose -f ./deploy/docker/docker-compose.yaml  up postgres redis -d


startf: # Start the frontend with templ
	templ generate --watch

migrate: # Run database migrations with tern
	 tern migrate \
    -c ./internal/store/migrations/tern.conf \
    --migrations ./internal/store/migrations/

sqlc: # Generate sqlc code
	@if command -v sqlc >/dev/null 2>&1; then \
		sqlc generate -f ./internal/store/sqlc.yaml; \
	elif [ -x "$$(go env GOPATH)/bin/sqlc" ]; then \
		"$$(go env GOPATH)/bin/sqlc" generate -f ./internal/store/sqlc.yaml; \
	else \
		echo "sqlc binary not found; running via go run..."; \
		go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate -f ./internal/store/sqlc.yaml; \
	fi

gen: # Generate sqlc code
	$(MAKE) sqlc && \
	templ generate ./...

setup: # Install necessary tools
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/jackc/tern/v2@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest


front: # Build and run the frontend with templ
	templ generate --watch --proxy="http://localhost:8080" --cmd="go run ./cmd/finops/main.go"

list: # List all available targets
	@echo "Available targets:"
	@awk '/^[A-Za-z0-9_.-]+:.*# / { i = index($$0, ":"); cmd = substr($$0, 1, i - 1); desc = substr($$0, i + 4); printf "%c[36m%-20s%c[0m %s\n", 27, cmd, 27, desc; }' $(MAKEFILE_LIST) | sort
