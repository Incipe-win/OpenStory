// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds all application configuration.
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Redis         RedisConfig
	Kafka         KafkaConfig
	MinIO         MinIOConfig
	JWT           JWTConfig
	Provider      ProviderConfig
	Limits        LimitsConfig
	Observability ObservabilityConfig
}

// JWTConfig holds JWT token settings.
type JWTConfig struct {
	Secret          string        `env:"JWT_SECRET"      envDefault:"openstory-dev-secret-change-in-production"`
	AccessTokenTTL  time.Duration `env:"JWT_ACCESS_TTL"  envDefault:"15m"`
	RefreshTokenTTL time.Duration `env:"JWT_REFRESH_TTL" envDefault:"168h"`
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
	Endpoint       string `env:"MINIO_ENDPOINT"        envDefault:"localhost:19000"`
	PublicEndpoint string `env:"MINIO_PUBLIC_ENDPOINT" envDefault:""`
	AccessKey      string `env:"MINIO_ACCESS_KEY"      envDefault:"minioadmin"`
	SecretKey      string `env:"MINIO_SECRET_KEY"      envDefault:"minioadmin"`
	Bucket         string `env:"MINIO_BUCKET"          envDefault:"openstory"`
	Region         string `env:"MINIO_REGION"          envDefault:"us-east-1"`
	UseSSL         bool   `env:"MINIO_USE_SSL"         envDefault:"false"`
}

type ProviderConfig struct {
	MaxAttempts             int    `env:"PROVIDER_MAX_ATTEMPTS"                  envDefault:"2"`
	ConfigEncryptionKey     string `env:"PROVIDER_CONFIG_ENCRYPTION_KEY"         envDefault:""`
	OpenAICompatibleBaseURL string `env:"OPENAI_COMPATIBLE_BASE_URL"            envDefault:"https://api.openai.com/v1"`
	OpenAICompatibleModel   string `env:"OPENAI_COMPATIBLE_MODEL"               envDefault:"gpt-4o-mini"`
	OpenAICompatibleAPIKey  string `env:"OPENAI_COMPATIBLE_API_KEY"             envDefault:""`
	DisableJSONSchema       bool   `env:"OPENAI_COMPATIBLE_DISABLE_JSON_SCHEMA" envDefault:"false"`
	OpenAIMaxTokens         int    `env:"OPENAI_COMPATIBLE_MAX_TOKENS"          envDefault:"4096"`
	OpenAIImageModel        string `env:"OPENAI_IMAGE_MODEL"                    envDefault:"dall-e-3"`
	OpenAIImageSize         string `env:"OPENAI_IMAGE_SIZE"                     envDefault:"1024x1024"`
	OpenAIImageQuality      string `env:"OPENAI_IMAGE_QUALITY"                  envDefault:"standard"`
	ComfyUIBaseURL          string `env:"COMFYUI_BASE_URL"                      envDefault:""`
	ComfyUIAPIKey           string `env:"COMFYUI_API_KEY"                       envDefault:""`
	ReplicateBaseURL        string `env:"REPLICATE_BASE_URL"                    envDefault:"https://api.replicate.com/v1"`
	ReplicateAPIToken       string `env:"REPLICATE_API_TOKEN"                   envDefault:""`
}

type LimitsConfig struct {
	UserRequestsPerMinute int `env:"RATE_LIMIT_USER_PER_MINUTE" envDefault:"120"`
	IPRequestsPerMinute   int `env:"RATE_LIMIT_IP_PER_MINUTE"   envDefault:"300"`
	TaskConcurrentLimit   int `env:"TASK_CONCURRENT_LIMIT"      envDefault:"5"`
}

type ObservabilityConfig struct {
	ServiceName     string `env:"OTEL_SERVICE_NAME"             envDefault:"openstory"`
	TracingEnabled  bool   `env:"OTEL_TRACES_ENABLED"          envDefault:"false"`
	OTLPEndpoint    string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"  envDefault:"localhost:4318"`
	OTLPInsecure    bool   `env:"OTEL_EXPORTER_OTLP_INSECURE"  envDefault:"true"`
	DiagnosticsAddr string `env:"OBSERVABILITY_ADDR"           envDefault:":9090"`
}

// Load parses environment variables into a Config struct.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}
