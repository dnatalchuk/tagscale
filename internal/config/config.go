// internal/config/config.go
package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                   int
	DatabaseURL            string
	AWSRegion              string
	AWSRequestTimeout      int // seconds
	SlackToken             string
	SlackChannel           string
	EmailSMTPHost          string
	EmailSMTPPort          int
	EmailUsername          string
	EmailPassword          string
	DataCollectionInterval int // minutes
	AnalysisInterval       int // minutes
	LogLevel               string
	APIKeys                []string
	AllowedOrigins         []string
	AllowNoAuth            bool
}

func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	config := &Config{
		Port:                   getEnvAsInt("PORT", 8080),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://user:password@localhost/tagscale?sslmode=disable"),
		AWSRegion:              getEnv("AWS_REGION", "us-east-1"),
		AWSRequestTimeout:      getEnvAsInt("AWS_REQUEST_TIMEOUT", 30),
		SlackToken:             getEnv("SLACK_TOKEN", ""),
		SlackChannel:           getEnv("SLACK_CHANNEL", "#general"),
		EmailSMTPHost:          getEnv("EMAIL_SMTP_HOST", ""),
		EmailSMTPPort:          getEnvAsInt("EMAIL_SMTP_PORT", 587),
		EmailUsername:          getEnv("EMAIL_USERNAME", ""),
		EmailPassword:          getEnv("EMAIL_PASSWORD", ""),
		DataCollectionInterval: getEnvAsInt("DATA_COLLECTION_INTERVAL", 60),
		AnalysisInterval:       getEnvAsInt("ANALYSIS_INTERVAL", 120),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		APIKeys:                getEnvAsSlice("API_KEYS", ",", []string{}),
		AllowedOrigins:         getEnvAsSlice("CORS_ORIGINS", ",", []string{"http://localhost:3000"}),
		AllowNoAuth:            getEnvAsBool("ALLOW_NO_AUTH", false),
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsSlice(key string, sep string, defaultValue []string) []string {
	if value, exists := os.LookupEnv(key); exists {
		parts := strings.Split(value, sep)
		var result []string
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
