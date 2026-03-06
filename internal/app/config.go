package app

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr         string
	DbURL            string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	SessionCookie    string
	CookieSecure     bool
	SessionTTL       time.Duration
	RememberMeTTL    time.Duration
	SlidingSessionTTL bool
}

func LoadConfig() Config {
	godotenv.Load()
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	sessionCookie := getEnv("SESSION_COOKIE_NAME", "finops_session")
	cookieSecure := getEnvBool("COOKIE_SECURE", false)
	redisDB := getEnvInt("REDIS_DB", 0)
	sessionTTL := getEnvDuration("SESSION_TTL", 30*time.Minute)
	rememberMeTTL := getEnvDuration("REMEMBER_ME_TTL", 7*24*time.Hour)
	slidingSessionTTL := getEnvBool("SLIDING_SESSION_TTL", true)

	return Config{
		HTTPAddr:          addr,
		DbURL:             dbURL,
		RedisAddr:         redisAddr,
		RedisPassword:     redisPassword,
		RedisDB:           redisDB,
		SessionCookie:     sessionCookie,
		CookieSecure:      cookieSecure,
		SessionTTL:        sessionTTL,
		RememberMeTTL:     rememberMeTTL,
		SlidingSessionTTL: slidingSessionTTL,
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
