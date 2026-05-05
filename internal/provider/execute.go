package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func Execute(ctx context.Context, p Provider, req Request, maxAttempts int) (*Result, error) {
	switch requestedCapability(req.TaskType) {
	case CapabilityImageGenerate:
		img, ok := p.(ImageGenerator)
		if !ok {
			return nil, ErrUnsupportedCapability
		}
		ir := buildImageRequest(req)
		res, err := img.ImageGenerate(ctx, ir)
		if err != nil {
			return nil, err
		}
		return &Result{Output: res.Output, CostCredits: res.CostCredits, Capability: CapabilityImageGenerate, Model: res.Model}, nil
	case CapabilityVideoGenerate:
		video, ok := p.(VideoGenerator)
		if !ok {
			return nil, ErrUnsupportedCapability
		}
		vr := buildVideoRequest(req)
		res, err := video.VideoGenerate(ctx, vr)
		if err != nil {
			return nil, err
		}
		return &Result{Output: res.Output, CostCredits: res.CostCredits, Capability: CapabilityVideoGenerate, Model: res.Model}, nil
	default:
		text, ok := p.(TextGenerator)
		if !ok {
			return nil, ErrUnsupportedCapability
		}
		textReq := BuildTextRequest(req, maxAttempts)
		res, err := executeStructuredText(ctx, text, textReq)
		if err != nil {
			return nil, err
		}
		return &Result{Output: res.Output, CostCredits: res.CostCredits, Capability: CapabilityTextGenerate, Model: res.Model, Usage: res.Usage}, nil
	}
}

func CapabilityForTask(taskType string) Capability {
	return requestedCapability(taskType)
}

func executeStructuredText(ctx context.Context, text TextGenerator, req TextRequest) (*TextResult, error) {
	attempts := req.MaxAttempts
	if attempts <= 0 {
		attempts = 2
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			req.UserPrompt += fmt.Sprintf("\n\nPrevious output failed schema validation: %v. Return corrected JSON only.", lastErr)
		}
		res, err := text.TextGenerate(ctx, req)
		if err != nil {
			lastErr = err
			continue
		}
		clean := cleanJSON(res.Output)
		if err := ValidateStructuredJSON(req.Schema, clean); err != nil {
			lastErr = err
			continue
		}
		res.Output = clean
		return res, nil
	}

	fallback := StructuredFallback(req.TaskType, req.Input)
	if err := ValidateStructuredJSON(req.Schema, fallback); err != nil {
		return nil, fmt.Errorf("fallback schema validation failed after provider errors: %w", err)
	}
	return &TextResult{
		Output:      fallback,
		CostCredits: 0,
		Model:       text.Name() + "-fallback",
	}, nil
}

func requestedCapability(taskType string) Capability {
	switch normalizeTaskType(taskType) {
	case TaskImageGeneration:
		return CapabilityImageGenerate
	case TaskVideoGeneration:
		return CapabilityVideoGenerate
	default:
		return CapabilityTextGenerate
	}
}

func buildImageRequest(req Request) ImageRequest {
	var input map[string]any
	_ = json.Unmarshal(req.Input, &input)
	return ImageRequest{
		TaskType: req.TaskType,
		Input:    req.Input,
		Prompt:   stringField(input, "prompt"),
		Model:    stringField(input, "model"),
	}
}

func buildVideoRequest(req Request) VideoRequest {
	var input map[string]any
	_ = json.Unmarshal(req.Input, &input)
	return VideoRequest{
		TaskType: req.TaskType,
		Input:    req.Input,
		Prompt:   stringField(input, "prompt"),
		ImageURL: stringField(input, "image_url"),
		Model:    stringField(input, "model"),
	}
}

func stringField(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	if value, ok := input[key].(string); ok {
		return value
	}
	return ""
}

func cleanJSON(raw json.RawMessage) json.RawMessage {
	text := strings.TrimSpace(string(raw))
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return json.RawMessage(strings.TrimSpace(text))
}
