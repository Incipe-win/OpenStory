// Package config loads application configuration from environment variables.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	MinIO    MinIOConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port    int    `env:"SERVER_PORT" envDefault:"8080"`
	Env     string `env:"APP_ENV"    envDefault:"development"`
	Version string `env:"APP_VERSION" envDefault:"0.1.0"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL string `env:"DATABASE_URL" envDefault:"postgres://openstory:openstory@localhost:15432/openstory?sslmode=disable"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string `env:"REDIS_ADDR"     envDefault:"localhost:16379"`
	Password string `env:"REDIS_PASSWORD" envDefault:""`
	DB       int    `env:"REDIS_DB"       envDefault:"0"`
}

// KafkaConfig holds Kafka broker settings.
type KafkaConfig struct {
	Brokers []string `env:"KAFKA_BROKERS" envSeparator:"," envDefault:"localhost:19092"`
}

// MinIOConfig holds MinIO/S3 connection settings.
type MinIOConfig struct {
	Endpoint  string `env:"MINIO_ENDPOINT"   envDefault:"localhost:19000"`
	AccessKey string `env:"MINIO_ACCESS_KEY" envDefault:"minioadmin"`
	SecretKey string `env:"MINIO_SECRET_KEY" envDefault:"minioadmin"`
	Bucket    string `env:"MINIO_BUCKET"     envDefault:"openstory"`
	UseSSL    bool   `env:"MINIO_USE_SSL"    envDefault:"false"`
}

// Load parses environment variables into a Config struct.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}
