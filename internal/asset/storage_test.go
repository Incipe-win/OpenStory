package asset

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Incipe-win/OpenStory/internal/config"
)

func TestResolveObjectURLPresignsS3URLWithPublicEndpoint(t *testing.T) {
	storage, err := NewStorage(config.MinIOConfig{
		Endpoint:       "minio:9000",
		PublicEndpoint: "https://incipe.top",
		AccessKey:      "minioadmin",
		SecretKey:      "minioadmin",
		Bucket:         "openstory",
	})
	if err != nil {
		t.Fatalf("NewStorage returned error: %v", err)
	}

	resolved, err := storage.ResolveObjectURL(
		context.Background(),
		"s3://openstory/projects/project-1/assets/asset-1/image.png",
		"",
		"",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("ResolveObjectURL returned error: %v", err)
	}
	if strings.HasPrefix(resolved, "s3://") {
		t.Fatalf("expected browser-safe URL, got %q", resolved)
	}
	u, err := url.Parse(resolved)
	if err != nil {
		t.Fatalf("resolved URL is invalid: %v", err)
	}
	if u.Scheme != "https" || u.Host != "incipe.top" {
		t.Fatalf("expected public https host, got %s://%s", u.Scheme, u.Host)
	}
	if u.Path != "/openstory/projects/project-1/assets/asset-1/image.png" {
		t.Fatalf("unexpected path: %s", u.Path)
	}
	if u.Query().Get("X-Amz-Signature") == "" {
		t.Fatalf("expected presigned URL query, got %q", resolved)
	}
}

func TestResolveObjectURLKeepsHTTPURL(t *testing.T) {
	storage := &Storage{}

	const existing = "https://cdn.example.com/image.png"
	resolved, err := storage.ResolveObjectURL(context.Background(), existing, "", "", time.Hour)
	if err != nil {
		t.Fatalf("ResolveObjectURL returned error: %v", err)
	}
	if resolved != existing {
		t.Fatalf("expected existing URL to be preserved, got %q", resolved)
	}
}
