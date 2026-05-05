package provider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SecretResolver interface {
	Resolve(ctx context.Context, providerName, configKey, envName string) (string, error)
}

type EnvThenDBSecretResolver struct {
	pool          *pgxpool.Pool
	encryptionKey string
}

func NewSecretResolver(pool *pgxpool.Pool, encryptionKey string) *EnvThenDBSecretResolver {
	return &EnvThenDBSecretResolver{pool: pool, encryptionKey: encryptionKey}
}

func (r *EnvThenDBSecretResolver) Resolve(ctx context.Context, providerName, configKey, envName string) (string, error) {
	if envName != "" {
		if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
			return value, nil
		}
	}
	if r == nil || r.pool == nil {
		return "", nil
	}
	var encrypted string
	err := r.pool.QueryRow(ctx,
		`SELECT encrypted_value FROM provider_configs WHERE provider_name = $1 AND config_key = $2`,
		providerName, configKey,
	).Scan(&encrypted)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query provider config: %w", err)
	}
	return decryptSecret(encrypted, r.encryptionKey)
}

func decryptSecret(encoded, key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("PROVIDER_CONFIG_ENCRYPTION_KEY is required to decrypt database provider config")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode encrypted provider config: %w", err)
	}
	if len(data) < 13 {
		return "", fmt.Errorf("encrypted provider config is too short")
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) <= nonceSize {
		return "", fmt.Errorf("encrypted provider config missing ciphertext")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt provider config: %w", err)
	}
	return string(plaintext), nil
}
