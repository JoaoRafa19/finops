package app

import "os"

type Config struct {
	HTTPAddr string
}

func LoadConfig() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		HTTPAddr: addr,
	}
}
