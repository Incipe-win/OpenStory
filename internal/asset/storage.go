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
	publicClient  *minio.Client
	bucket        string
	publicBaseURL string
}

func NewStorage(cfg config.MinIOConfig) (*Storage, error) {
	region := storageRegion(cfg.Region)
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	s := &Storage{client: client, bucket: cfg.Bucket}
	if cfg.PublicEndpoint != "" {
		publicEndpoint := strings.TrimRight(cfg.PublicEndpoint, "/")
		s.publicBaseURL = publicEndpoint + "/" + cfg.Bucket
		publicClient, err := newPublicPresignClient(publicEndpoint, cfg)
		if err != nil {
			return nil, err
		}
		s.publicClient = publicClient
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
	client := s.client
	if s.publicClient != nil {
		client = s.publicClient
	}
	u, err := client.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ResolveObjectURL converts an internal s3:// asset location into a browser-safe URL.
// It preserves existing http(s) URLs, signs private MinIO/S3 objects when possible,
// and falls back to the configured public object prefix for public buckets/proxies.
func (s *Storage) ResolveObjectURL(ctx context.Context, rawURL, bucket, key string, expiry time.Duration) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" && key == "" {
		return "", nil
	}
	if isHTTPURL(rawURL) {
		return rawURL, nil
	}
	if parsedBucket, parsedKey, ok := parseObjectURL(rawURL); ok {
		if bucket == "" {
			bucket = parsedBucket
		}
		if key == "" {
			key = parsedKey
		}
	}
	if key == "" {
		return rawURL, nil
	}
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	if s == nil || s.client == nil {
		return "", fmt.Errorf("object storage is not configured")
	}
	if bucket == "" {
		bucket = s.bucket
	}
	client := s.client
	if s.publicClient != nil {
		client = s.publicClient
	}
	if client != nil {
		u, err := client.PresignedGetObject(ctx, bucket, key, expiry, url.Values{})
		if err == nil {
			return u.String(), nil
		}
		if s.publicBaseURL == "" {
			return "", err
		}
	}
	if s.publicBaseURL != "" {
		return s.publicBaseURL + "/" + strings.TrimLeft(key, "/"), nil
	}
	return rawURL, nil
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

func newPublicPresignClient(publicEndpoint string, cfg config.MinIOConfig) (*minio.Client, error) {
	u, err := url.Parse(publicEndpoint)
	if err != nil {
		return nil, fmt.Errorf("parse MINIO_PUBLIC_ENDPOINT: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("MINIO_PUBLIC_ENDPOINT must start with http:// or https://")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("MINIO_PUBLIC_ENDPOINT must include a host")
	}
	return minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: u.Scheme == "https",
		Region: storageRegion(cfg.Region),
	})
}

func storageRegion(region string) string {
	if region == "" {
		return "us-east-1"
	}
	return region
}

func isHTTPURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")
}

func parseObjectURL(rawURL string) (string, string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "s3" || u.Host == "" {
		return "", "", false
	}
	key := strings.TrimLeft(u.Path, "/")
	if key == "" {
		return "", "", false
	}
	return u.Host, key, true
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
