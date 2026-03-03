APP_NAME=finops
CMD_PATH=./cmd/finops/

.PHONY: run build test fmt tidy

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
