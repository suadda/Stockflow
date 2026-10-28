package config

import "os"

type Config struct {
	Addr string

	DatabaseURL string
}

const (
	defaultAddr        = ":8080"
	defaultDatabaseURL = "postgres://stockflow:stockflow@localhost:5432/stockflow?sslmode=disable"
)

func Load() Config {
	return Config{
		Addr:        getenv("ADDR", defaultAddr),
		DatabaseURL: getenv("DATABASE_URL", defaultDatabaseURL),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
