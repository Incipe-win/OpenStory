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

func (m *MockProvider) Supports(taskType string) bool { return true }

func (m *MockProvider) Generate(ctx context.Context, req Request) (*Result, error) {
	// Simulate processing time (500ms - 3s)
	delay := time.Duration(500+rand.Intn(2500)) * time.Millisecond

	select {
	case <-time.After(delay):
		// Success
	case <-ctx.Done():
		return nil, fmt.Errorf("task canceled: %w", ctx.Err())
	}

	// Generate mock output based on task type
	output := map[string]any{
		"provider":   "mock",
		"task_type":  req.TaskType,
		"generated":  true,
		"mock_data":  generateMockData(req.TaskType),
		"elapsed_ms": delay.Milliseconds(),
	}
	outputJSON, _ := json.Marshal(output)

	// Mock cost: 1-10 credits depending on type
	cost := mockCost(req.TaskType)

	return &Result{
		Output:      outputJSON,
		CostCredits: cost,
	}, nil
}

func generateMockData(taskType string) any {
	switch taskType {
	case "script":
		return map[string]any{
			"title":    "Mock Story",
			"scenes":   3,
			"duration": "30s",
		}
	case "character":
		return []map[string]any{
			{"name": "Hero", "description": "A brave protagonist"},
			{"name": "Sidekick", "description": "A loyal companion"},
		}
	case "scene":
		return []map[string]any{
			{"id": 1, "description": "Opening scene in a forest"},
			{"id": 2, "description": "Confrontation at the bridge"},
			{"id": 3, "description": "Resolution at sunset"},
		}
	case "image_generation":
		return map[string]any{
			"image_url": "https://mock.openstory.local/images/mock-001.png",
			"width":     1024,
			"height":    768,
		}
	case "video_generation":
		return map[string]any{
			"video_url": "https://mock.openstory.local/videos/mock-001.mp4",
			"duration":  "5s",
		}
	case "audio":
		return map[string]any{
			"audio_url": "https://mock.openstory.local/audio/mock-001.mp3",
			"duration":  "30s",
		}
	case "compose":
		return map[string]any{
			"video_url": "https://mock.openstory.local/final/mock-final.mp4",
			"duration":  "30s",
			"format":    "mp4",
		}
	default:
		return map[string]any{"result": "mock output for " + taskType}
	}
}

func mockCost(taskType string) int {
	costs := map[string]int{
		"script":           2,
		"character":        1,
		"scene":            1,
		"storyboard":       2,
		"image_prompt":     1,
		"image_generation": 5,
		"video_generation": 10,
		"audio":            3,
		"subtitle":         1,
		"compose":          5,
	}
	if c, ok := costs[taskType]; ok {
		return c
	}
	return 1
}
