package gen


//go:generate sqlc generate -f ./internal/store/sqlc.yaml

//go:generate templ generate ./...
