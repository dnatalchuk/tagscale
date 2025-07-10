/ internal/config/config.go
package config

import (
    "os"
    "strconv"

    "github.com/joho/godotenv"
)

type Config struct {
    Port                    int
    DatabaseURL             string
    AWSRegion               string
    SlackToken              string
    SlackChannel            string
    EmailSMTPHost           string
    EmailSMTPPort           int
    EmailUsername           string
    EmailPassword           string
    DataCollectionInterval  int // minutes
    AnalysisInterval        int // minutes
    LogLevel                string
}

func Load() (*Config, error) {
    // Load .env file if it exists
    _ = godotenv.Load()

    config := &Config{
        Port:                   getEnvAsInt("PORT", 8080),
        DatabaseURL:            getEnv("DATABASE_URL", "postgres://user:password@localhost/tagscale?sslmode=disable"),
        AWSRegion:              getEnv("AWS_REGION", "us-east-1"),
        SlackToken:             getEnv("SLACK_TOKEN", ""),
        SlackChannel:           getEnv("SLACK_CHANNEL", "#general"),
        EmailSMTPHost:          getEnv("EMAIL_SMTP_HOST", ""),
        EmailSMTPPort:          getEnvAsInt("EMAIL_SMTP_PORT", 587),
        EmailUsername:          getEnv("EMAIL_USERNAME", ""),
        EmailPassword:          getEnv("EMAIL_PASSWORD", ""),
        DataCollectionInterval: getEnvAsInt("DATA_COLLECTION_INTERVAL", 60),
        AnalysisInterval:       getEnvAsInt("ANALYSIS_INTERVAL", 120),
        LogLevel:               getEnv("LOG_LEVEL", "info"),
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
