package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "development", cfg.Server.Env)
	assert.Equal(t, "0.1.0", cfg.Server.Version)
	assert.Contains(t, cfg.Database.URL, "openstory")
	assert.Equal(t, "localhost:16379", cfg.Redis.Addr)
	assert.Equal(t, []string{"localhost:19092"}, cfg.Kafka.Brokers)
	assert.Equal(t, "openstory", cfg.MinIO.Bucket)
	assert.Equal(t, "us-east-1", cfg.MinIO.Region)
	assert.Equal(t, 120, cfg.Limits.UserRequestsPerMinute)
	assert.Equal(t, 300, cfg.Limits.IPRequestsPerMinute)
	assert.Equal(t, 5, cfg.Limits.TaskConcurrentLimit)
	assert.Equal(t, "openstory", cfg.Observability.ServiceName)
	assert.Equal(t, ":9090", cfg.Observability.DiagnosticsAddr)
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("SERVER_PORT")
	defer os.Unsetenv("APP_ENV")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "production", cfg.Server.Env)
}
