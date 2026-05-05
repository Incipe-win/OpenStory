package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Incipe-win/OpenStory/internal/asset"
	"github.com/Incipe-win/OpenStory/internal/observability"
)

type AssetRepository interface {
	Create(ctx context.Context, a *asset.Asset) error
	Get(ctx context.Context, id uuid.UUID) (*asset.Asset, error)
}

type ObjectStorage interface {
	Bucket() string
	FGetObject(ctx context.Context, key, filePath string) error
	FPutObject(ctx context.Context, key, filePath, contentType string) (any, error)
}

type MinIOStorage interface {
	Bucket() string
	FGetObject(ctx context.Context, key, filePath string) error
	FPutObject(ctx context.Context, key, filePath, contentType string) (info any, err error)
}

type Service struct {
	assets  AssetRepository
	storage storageClient
	ffmpeg  string
}

type storageClient interface {
	Bucket() string
	FGetObject(ctx context.Context, key, filePath string) error
	FPutObject(ctx context.Context, key, filePath, contentType string) (any, error)
}

type minioStorageAdapter struct {
	storage *asset.Storage
}

func (a minioStorageAdapter) Bucket() string { return a.storage.Bucket() }
func (a minioStorageAdapter) FGetObject(ctx context.Context, key, filePath string) error {
	return a.storage.FGetObject(ctx, key, filePath)
}
func (a minioStorageAdapter) FPutObject(ctx context.Context, key, filePath, contentType string) (any, error) {
	return a.storage.FPutObject(ctx, key, filePath, contentType)
}

func NewService(assets AssetRepository, storage *asset.Storage) *Service {
	return &Service{assets: assets, storage: minioStorageAdapter{storage: storage}, ffmpeg: "ffmpeg"}
}

type Input struct {
	ImageAssetIDs      []uuid.UUID `json:"image_asset_ids"`
	Subtitle           string      `json:"subtitle"`
	SubtitleFormat     string      `json:"subtitle_format"`
	DurationPerImageMs int         `json:"duration_per_image_ms"`
	Width              int         `json:"width"`
	Height             int         `json:"height"`
	FPS                int         `json:"fps"`
	Title              string      `json:"title"`
}

type Result struct {
	VideoAssetID    uuid.UUID  `json:"video_asset_id"`
	CoverAssetID    uuid.UUID  `json:"cover_asset_id"`
	SubtitleAssetID *uuid.UUID `json:"subtitle_asset_id,omitempty"`
	VideoURL        string     `json:"video_url"`
	CoverURL        string     `json:"cover_url"`
	DurationMs      int        `json:"duration_ms"`
	Width           int        `json:"width"`
	Height          int        `json:"height"`
}

func ParseInput(data json.RawMessage) (Input, error) {
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return input, fmt.Errorf("decode compose input: %w", err)
	}
	if input.DurationPerImageMs <= 0 {
		input.DurationPerImageMs = 3000
	}
	if input.Width <= 0 {
		input.Width = 1280
	}
	if input.Height <= 0 {
		input.Height = 720
	}
	if input.FPS <= 0 {
		input.FPS = 30
	}
	if input.SubtitleFormat == "" {
		input.SubtitleFormat = "srt"
	}
	input.SubtitleFormat = strings.ToLower(input.SubtitleFormat)
	if input.SubtitleFormat != "srt" && input.SubtitleFormat != "vtt" {
		return input, fmt.Errorf("subtitle_format must be srt or vtt")
	}
	if len(input.ImageAssetIDs) == 0 {
		return input, fmt.Errorf("image_asset_ids is required")
	}
	return input, nil
}

func (s *Service) Compose(ctx context.Context, taskID, userID, projectID uuid.UUID, inputJSON json.RawMessage) (*Result, error) {
	input, err := ParseInput(inputJSON)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "openstory-compose-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	imageFiles := make([]string, 0, len(input.ImageAssetIDs))
	for i, imageID := range input.ImageAssetIDs {
		a, err := s.assets.Get(ctx, imageID)
		if err != nil {
			return nil, err
		}
		if a.UserID != userID || a.ProjectID == nil || *a.ProjectID != projectID {
			return nil, fmt.Errorf("image asset %s not found", imageID)
		}
		ext := extensionForMime(a.MimeType)
		imagePath := filepath.Join(tmpDir, fmt.Sprintf("image-%03d%s", i, ext))
		if err := s.storage.FGetObject(ctx, a.StorageKey, imagePath); err != nil {
			return nil, fmt.Errorf("download image asset %s: %w", imageID, err)
		}
		imageFiles = append(imageFiles, imagePath)
	}

	durationMs := input.DurationPerImageMs * len(imageFiles)
	listPath := filepath.Join(tmpDir, "images.txt")
	if err := writeConcatList(listPath, imageFiles, input.DurationPerImageMs); err != nil {
		return nil, err
	}

	videoPath := filepath.Join(tmpDir, "output.mp4")
	if err := s.runFFmpeg(ctx, "compose_video", "-y", "-f", "concat", "-safe", "0", "-i", listPath,
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,format=yuv420p", input.Width, input.Height, input.Width, input.Height),
		"-r", fmt.Sprintf("%d", input.FPS), "-movflags", "+faststart", videoPath); err != nil {
		return nil, err
	}

	coverPath := filepath.Join(tmpDir, "cover.jpg")
	if err := s.runFFmpeg(ctx, "cover_extract", "-y", "-i", videoPath, "-frames:v", "1", coverPath); err != nil {
		return nil, err
	}

	videoKey := asset.BuildComposeKey(projectID.String(), taskID.String(), "output.mp4")
	coverKey := asset.BuildComposeKey(projectID.String(), taskID.String(), "cover.jpg")
	if _, err := s.storage.FPutObject(ctx, videoKey, videoPath, "video/mp4"); err != nil {
		return nil, fmt.Errorf("upload composed video: %w", err)
	}
	if _, err := s.storage.FPutObject(ctx, coverKey, coverPath, "image/jpeg"); err != nil {
		return nil, fmt.Errorf("upload cover: %w", err)
	}

	videoAsset, err := s.createAsset(ctx, userID, projectID, "video", firstNonEmpty(input.Title, "Composed video"), "video/mp4", videoPath, videoKey, durationMs, input.Width, input.Height)
	if err != nil {
		return nil, err
	}
	coverAsset, err := s.createAsset(ctx, userID, projectID, "cover", "Video cover", "image/jpeg", coverPath, coverKey, 0, input.Width, input.Height)
	if err != nil {
		return nil, err
	}

	var subtitleAssetID *uuid.UUID
	if strings.TrimSpace(input.Subtitle) != "" {
		subtitleID, err := s.createSubtitleAsset(ctx, tmpDir, taskID, userID, projectID, input)
		if err != nil {
			return nil, err
		}
		subtitleAssetID = &subtitleID
	}

	return &Result{
		VideoAssetID:    videoAsset.ID,
		CoverAssetID:    coverAsset.ID,
		SubtitleAssetID: subtitleAssetID,
		VideoURL:        videoAsset.URL,
		CoverURL:        coverAsset.URL,
		DurationMs:      durationMs,
		Width:           input.Width,
		Height:          input.Height,
	}, nil
}

func (s *Service) createAsset(ctx context.Context, userID, projectID uuid.UUID, assetType, name, mimeType, filePath, key string, durationMs, width, height int) (*asset.Asset, error) {
	size, err := asset.FileSize(filePath)
	if err != nil {
		return nil, err
	}
	checksum, err := asset.ChecksumSHA256(filePath)
	if err != nil {
		return nil, err
	}
	a := &asset.Asset{
		UserID:        userID,
		ProjectID:     &projectID,
		Type:          assetType,
		Name:          name,
		MimeType:      mimeType,
		SizeBytes:     size,
		DurationMs:    intPtr(durationMs),
		Width:         intPtr(width),
		Height:        intPtr(height),
		Checksum:      checksum,
		StorageKey:    key,
		StorageBucket: s.storage.Bucket(),
		URL:           asset.ObjectURL(s.storage.Bucket(), key),
		MetadataJSON:  json.RawMessage(`{}`),
	}
	if durationMs == 0 {
		a.DurationMs = nil
	}
	if err := s.assets.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) createSubtitleAsset(ctx context.Context, tmpDir string, taskID, userID, projectID uuid.UUID, input Input) (uuid.UUID, error) {
	ext := "." + input.SubtitleFormat
	mimeType := "application/x-subrip"
	if input.SubtitleFormat == "vtt" {
		mimeType = "text/vtt"
	}
	subtitlePath := filepath.Join(tmpDir, "subtitles"+ext)
	if err := os.WriteFile(subtitlePath, []byte(input.Subtitle), 0o600); err != nil {
		return uuid.Nil, err
	}
	key := asset.BuildComposeKey(projectID.String(), taskID.String(), "subtitles"+ext)
	if _, err := s.storage.FPutObject(ctx, key, subtitlePath, mimeType); err != nil {
		return uuid.Nil, err
	}
	a, err := s.createAsset(ctx, userID, projectID, "subtitle", "Subtitles", mimeType, subtitlePath, key, 0, 0, 0)
	if err != nil {
		return uuid.Nil, err
	}
	return a.ID, nil
}

func writeConcatList(path string, files []string, durationPerImageMs int) error {
	var b strings.Builder
	seconds := float64(durationPerImageMs) / 1000
	for _, file := range files {
		b.WriteString("file '")
		b.WriteString(strings.ReplaceAll(file, "'", "'\\''"))
		b.WriteString("'\n")
		b.WriteString(fmt.Sprintf("duration %.3f\n", seconds))
	}
	b.WriteString("file '")
	b.WriteString(strings.ReplaceAll(files[len(files)-1], "'", "'\\''"))
	b.WriteString("'\n")
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func (s *Service) runFFmpeg(ctx context.Context, operation string, args ...string) error {
	bin := s.ffmpeg
	if bin == "" {
		bin = "ffmpeg"
	}
	started := time.Now()
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		observability.ObserveFFmpeg(operation, "failed", time.Since(started))
		return fmt.Errorf("ffmpeg failed: %w: %s", err, string(out))
	}
	observability.ObserveFFmpeg(operation, "succeeded", time.Since(started))
	return nil
}

func extensionForMime(mimeType string) string {
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func intPtr(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
