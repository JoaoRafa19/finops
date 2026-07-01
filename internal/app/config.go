package app

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr          string
	DbURL             string
	RedisURL          string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	SessionCookie     string
	CookieSecure      bool
	SessionTTL        time.Duration
	RememberMeTTL     time.Duration
	SlidingSessionTTL bool
	LogLevel          slog.Level
	LLMBaseURL       string
	LLMAPIKey        string
	LLMModel         string
	EmbeddingBaseURL string
	EmbeddingAPIKey  string
	EmbeddingModel   string
	AppBaseURL       string
	SMTPHost         string
	SMTPPort         string
	SMTPUser         string
	SMTPPassword     string
	SMTPFrom         string
	ResendAPIKey     string
	ResendFrom       string
	MigrateOnStart   bool
}

func LoadConfig() Config {
	godotenv.Load()
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	redisAddr := strings.TrimSpace(getEnv("REDIS_ADDR", "localhost:6379"))
	redisPassword := strings.TrimSpace(os.Getenv("REDIS_PASSWORD"))
	sessionCookie := strings.TrimSpace(getEnv("SESSION_COOKIE_NAME", "finops_session"))
	cookieSecure := getEnvBool("COOKIE_SECURE", false)
	redisDB := getEnvInt("REDIS_DB", 0)
	sessionTTL := getEnvDuration("SESSION_TTL", 30*time.Minute)
	rememberMeTTL := getEnvDuration("REMEMBER_ME_TTL", 7*24*time.Hour)
	slidingSessionTTL := getEnvBool("SLIDING_SESSION_TTL", true)
	logLevel := getEnvLogLevel("LOG_LEVEL", slog.LevelInfo)

	return Config{
		HTTPAddr:          addr,
		DbURL:             dbURL,
		RedisURL:          strings.TrimSpace(os.Getenv("REDIS_URL")),
		RedisAddr:         redisAddr,
		RedisPassword:     redisPassword,
		RedisDB:           redisDB,
		SessionCookie:     sessionCookie,
		CookieSecure:      cookieSecure,
		SessionTTL:        sessionTTL,
		RememberMeTTL:     rememberMeTTL,
		SlidingSessionTTL: slidingSessionTTL,
		LogLevel:          logLevel,
		LLMBaseURL:       getEnv("LLM_BASE_URL", getEnv("OLLAMA_BASE_URL", "http://localhost:11434")),
		LLMAPIKey:        getEnv("LLM_API_KEY", "ollama"),
		LLMModel:         getEnv("LLM_MODEL", getEnv("OLLAMA_MODEL", "qwen2.5:3b")),
		EmbeddingBaseURL: getEnv("EMBEDDING_BASE_URL", getEnv("LLM_BASE_URL", "http://localhost:11434")),
		EmbeddingAPIKey:  getEnv("EMBEDDING_API_KEY", getEnv("LLM_API_KEY", "ollama")),
		EmbeddingModel:   getEnv("EMBEDDING_MODEL", "nomic-embed-text"),
		AppBaseURL:       getEnv("APP_BASE_URL", "http://localhost:8080"),
		SMTPHost:         getEnv("SMTP_HOST", ""),
		SMTPPort:         getEnv("SMTP_PORT", "587"),
		SMTPUser:         getEnv("SMTP_USER", ""),
		SMTPPassword:     getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:         getEnv("SMTP_FROM", ""),
		ResendAPIKey:     getEnv("RESEND_API_KEY", ""),
		ResendFrom:       getEnv("RESEND_FROM", "onboarding@resend.dev"),
		MigrateOnStart:   getEnvBool("MIGRATE_ON_START", false),
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
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

func getEnvLogLevel(key string, fallback slog.Level) slog.Level {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return fallback
	}

	return level
}
