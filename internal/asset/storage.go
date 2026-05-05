package asset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/Incipe-win/OpenStory/internal/config"
)

type Storage struct {
	client        *minio.Client
	bucket        string
	publicBaseURL string
}

func NewStorage(cfg config.MinIOConfig) (*Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	s := &Storage{client: client, bucket: cfg.Bucket}
	if cfg.PublicEndpoint != "" {
		s.publicBaseURL = strings.TrimRight(cfg.PublicEndpoint, "/") + "/" + cfg.Bucket
	}
	return s, nil
}

func (s *Storage) Bucket() string {
	return s.bucket
}

func (s *Storage) PresignedPutURL(ctx context.Context, key, contentType string, expiry time.Duration) (*url.URL, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("object storage is not configured")
	}
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return nil, err
	}
	if s.publicBaseURL != "" {
		u.Path = "/" + s.bucket + "/" + key
		u.Host = ""
		u, err = url.Parse(s.publicBaseURL + "/" + key + "?" + u.RawQuery)
		if err != nil {
			return nil, err
		}
	}
	return u, nil
}

func (s *Storage) FGetObject(ctx context.Context, key, filePath string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("object storage is not configured")
	}
	return s.client.FGetObject(ctx, s.bucket, key, filePath, minio.GetObjectOptions{})
}

func (s *Storage) FPutObject(ctx context.Context, key, filePath, contentType string) (minio.UploadInfo, error) {
	if s == nil || s.client == nil {
		return minio.UploadInfo{}, fmt.Errorf("object storage is not configured")
	}
	return s.client.FPutObject(ctx, s.bucket, key, filePath, minio.PutObjectOptions{ContentType: contentType})
}

func ObjectURL(bucket, key string) string {
	return "s3://" + bucket + "/" + key
}

func BuildStorageKey(projectID, assetID, name string) string {
	clean := sanitizeName(name)
	if clean == "" {
		clean = "asset"
	}
	return path.Join("projects", projectID, "assets", assetID, clean)
}

func BuildComposeKey(projectID, taskID, name string) string {
	return path.Join("projects", projectID, "compose", taskID, sanitizeName(name))
}

func ChecksumSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func FileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

var unsafeName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "-")
	name = unsafeName.ReplaceAllString(name, "-")
	return strings.Trim(name, ".-")
}
