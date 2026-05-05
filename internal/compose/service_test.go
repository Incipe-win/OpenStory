package compose

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/Incipe-win/OpenStory/internal/asset"
)

func TestParseInputDefaults(t *testing.T) {
	imageID := uuid.New()
	input, err := ParseInput(mustJSON(t, map[string]any{
		"image_asset_ids": []string{imageID.String()},
	}))
	if err != nil {
		t.Fatalf("ParseInput returned error: %v", err)
	}
	if input.DurationPerImageMs != 3000 || input.Width != 1280 || input.Height != 720 || input.FPS != 30 {
		t.Fatalf("unexpected defaults: %#v", input)
	}
	if input.SubtitleFormat != "srt" {
		t.Fatalf("subtitle format = %q", input.SubtitleFormat)
	}
}

func TestParseInputRejectsMissingImages(t *testing.T) {
	_, err := ParseInput(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected missing image_asset_ids error")
	}
}

func TestStorageKeyAndChecksum(t *testing.T) {
	key := asset.BuildStorageKey("project-1", "asset-1", "../My Image 01.png")
	if key != "projects/project-1/assets/asset-1/My-Image-01.png" {
		t.Fatalf("unexpected key: %s", key)
	}

	f, err := os.CreateTemp("", "checksum-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("openstory"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	sum, err := asset.ChecksumSHA256(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if sum != "fa8e3bc21ee8f3641e16d8d7c6b66a23731707248f267c537d312d2737a45cd2" {
		t.Fatalf("unexpected checksum: %s", sum)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
