package main

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL        string
	OllamaBaseURL      string
	OllamaEmbedModel   string
	OllamaGenModel     string
	Port               string
	ChunkSize          int
	ChunkOverlap       int
	ConcurrencyWorkers int
}

func LoadConfig() Config {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		dbUser := getEnv("DB_USER", "user_db")
		dbPassword := os.Getenv("DB_PASSWORD")
		dbHost := getEnv("DB_HOST", "localhost")
		dbPort := getEnv("DB_PORT", "5432")
		dbName := getEnv("DB_NAME", "rag")
		dbSSLMode := getEnv("DB_SSLMODE", "disable")

		if dbPassword != "" {
			databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", dbUser, dbPassword, dbHost, dbPort, dbName, dbSSLMode)
		} else {
			databaseURL = fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s", dbUser, dbHost, dbPort, dbName, dbSSLMode)
		}
	}

	chunkSize, _ := strconv.Atoi(getEnv("CHUNK_SIZE", "1200"))
	if chunkSize <= 0 {
		chunkSize = 1200
	}

	chunkOverlap, _ := strconv.Atoi(getEnv("CHUNK_OVERLAP", "150"))
	if chunkOverlap < 0 {
		chunkOverlap = 150
	}

	workers, _ := strconv.Atoi(getEnv("CONCURRENCY_WORKERS", "6"))
	if workers <= 0 {
		workers = 6
	}

	return Config{
		DatabaseURL:        databaseURL,
		OllamaBaseURL:      getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaEmbedModel:   getEnv("OLLAMA_EMBED_MODEL", "nomic-embed-text:latest"),
		OllamaGenModel:     getEnv("OLLAMA_GEN_MODEL", "qwen3.5:2b"),
		Port:               getEnv("PORT", "8080"),
		ChunkSize:          chunkSize,
		ChunkOverlap:       chunkOverlap,
		ConcurrencyWorkers: workers,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
