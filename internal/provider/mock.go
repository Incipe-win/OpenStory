package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// MockProvider simulates AI generation with random delays.
type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Capabilities() []Capability {
	return []Capability{CapabilityTextGenerate, CapabilityImageGenerate, CapabilityVideoGenerate, CapabilityEmbedding}
}

func (m *MockProvider) TextGenerate(ctx context.Context, req TextRequest) (*TextResult, error) {
	delay := time.Duration(500+rand.Intn(2500)) * time.Millisecond
	if err := sleepOrCancel(ctx, delay); err != nil {
		return nil, err
	}

	output := mockStructuredOutput(req.TaskType, req.Input)
	return &TextResult{
		Output:      output,
		CostCredits: mockCost(req.TaskType),
		Model:       "mock-text",
		Usage: Usage{
			PromptTokens:     100,
			CompletionTokens: len(output) / 4,
			TotalTokens:      100 + len(output)/4,
		},
	}, nil
}

func (m *MockProvider) ImageGenerate(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	if err := sleepOrCancel(ctx, 500*time.Millisecond); err != nil {
		return nil, err
	}
	output := mustJSON(map[string]any{
		"images": []map[string]any{
			{"url": "https://mock.openstory.local/images/mock-001.png", "width": 1024, "height": 768, "prompt": req.Prompt},
		},
	})
	return &ImageResult{Output: output, CostCredits: mockCost(TaskImageGeneration), Model: "mock-image"}, nil
}

func (m *MockProvider) VideoGenerate(ctx context.Context, req VideoRequest) (*VideoResult, error) {
	if err := sleepOrCancel(ctx, 750*time.Millisecond); err != nil {
		return nil, err
	}
	output := mustJSON(map[string]any{
		"videos": []map[string]any{
			{"url": "https://mock.openstory.local/videos/mock-001.mp4", "duration_seconds": 5, "prompt": req.Prompt, "source_image_url": req.ImageURL},
		},
	})
	return &VideoResult{Output: output, CostCredits: mockCost(TaskVideoGeneration), Model: "mock-video"}, nil
}

func (m *MockProvider) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResult, error) {
	if err := sleepOrCancel(ctx, 100*time.Millisecond); err != nil {
		return nil, err
	}
	return &EmbeddingResult{Vectors: [][]float32{{0.1, 0.2, 0.3}}, Model: "mock-embedding"}, nil
}

func sleepOrCancel(ctx context.Context, delay time.Duration) error {
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("task canceled: %w", ctx.Err())
	}
}

func mockStructuredOutput(taskType string, input json.RawMessage) json.RawMessage {
	return StructuredFallback(taskType, input)
}

func mockCost(taskType string) int {
	costs := map[string]int{
		"idea":              1,
		"script":            2,
		"character":         1,
		"characters":        1,
		"scene":             1,
		"storyboard":        2,
		"image_prompt":      1,
		"video_prompt":      1,
		"creative_pipeline": 5,
		"image_generation":  5,
		"video_generation":  10,
		"audio":             3,
		"subtitle":          1,
		"compose":           5,
	}
	if c, ok := costs[taskType]; ok {
		return c
	}
	return 1
}
