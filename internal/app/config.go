package app

import "os"

type Config struct {
	HTTPAddr string
	DbURL    string
}

func LoadConfig() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	dbURL := os.Getenv("DATABASE_URL")

	return Config{
		HTTPAddr: addr,
		DbURL:    dbURL,
	}
}
