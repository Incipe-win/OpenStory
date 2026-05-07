package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider implements both TextGenerator and ImageGenerator using the OpenAI API.
type OpenAIProvider struct {
	baseURL           string
	apiKey            string
	model             string
	disableJSONSchema bool
	maxTokens         int
	imageModel        string
	imageSize         string
	imageQuality      string
	client            *http.Client
}

func NewOpenAIProvider(baseURL, apiKey, model string, disableJSONSchema bool, maxTokens int, imageModel, imageSize, imageQuality string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	if imageModel == "" {
		imageModel = "dall-e-3"
	}
	if imageSize == "" {
		imageSize = "1024x1024"
	}
	if imageQuality == "" {
		imageQuality = "standard"
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	return &OpenAIProvider{
		baseURL:           strings.TrimRight(baseURL, "/"),
		apiKey:            apiKey,
		model:             model,
		disableJSONSchema: disableJSONSchema,
		maxTokens:         maxTokens,
		imageModel:        imageModel,
		imageSize:         imageSize,
		imageQuality:      imageQuality,
		client:            &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string { return "openai-compatible" }

func (p *OpenAIProvider) Capabilities() []Capability {
	return []Capability{CapabilityTextGenerate, CapabilityImageGenerate}
}

// ── Text Generation ──────────────────────────────────

func (p *OpenAIProvider) TextGenerate(ctx context.Context, req TextRequest) (*TextResult, error) {
	if p.apiKey == "" {
		return nil, ErrProviderNotConfigured
	}

	messages := []map[string]string{
		{"role": "system", "content": req.SystemPrompt},
	}

	userContent := req.UserPrompt
	if p.disableJSONSchema {
		// Some models (e.g. Doubao/Volcano ARK) don't support response_format.
		// When JSON schema is disabled, embed the schema into the user prompt.
		userContent += fmt.Sprintf("\n\nYou must return ONLY valid JSON matching this schema (no markdown, no code fences):\n%s", string(req.Schema))
	}

	messages = append(messages, map[string]string{"role": "user", "content": userContent})

	body := map[string]any{
		"model":       p.model,
		"messages":    messages,
		"temperature": 0.2,
		"max_tokens":  p.maxTokens,
	}

	if !p.disableJSONSchema {
		var schema map[string]any
		if err := json.Unmarshal(req.Schema, &schema); err != nil {
			return nil, fmt.Errorf("decode schema for provider request: %w", err)
		}
		body["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   req.SchemaName,
				"strict": true,
				"schema": schema,
			},
		}
	}

	data, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call openai-compatible text provider: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai-compatible provider returned %d: %s", resp.StatusCode, string(respBody))
	}

	var decoded struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode openai-compatible response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return nil, fmt.Errorf("openai-compatible response had no choices")
	}

	output := json.RawMessage(strings.TrimSpace(decoded.Choices[0].Message.Content))
	return &TextResult{
		Output:      output,
		CostCredits: estimateTextCostCredits(decoded.Usage.TotalTokens),
		Model:       firstNonEmpty(decoded.Model, p.model),
		Usage: Usage{
			PromptTokens:     decoded.Usage.PromptTokens,
			CompletionTokens: decoded.Usage.CompletionTokens,
			TotalTokens:      decoded.Usage.TotalTokens,
		},
	}, nil
}

// ── Image Generation (DALL-E) ────────────────────────

func (p *OpenAIProvider) ImageGenerate(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	if p.apiKey == "" {
		return nil, ErrProviderNotConfigured
	}

	prompt := req.Prompt
	if prompt == "" {
		var input map[string]any
		_ = json.Unmarshal(req.Input, &input)
		prompt = stringField(input, "prompt")
	}
	if prompt == "" {
		prompt = "cinematic frame, detailed composition, natural light, photorealistic, 4k"
	}

	body := map[string]any{
		"model":   p.imageModel,
		"prompt":  prompt,
		"n":       1,
		"size":    p.imageSize,
		"quality": p.imageQuality,
	}
	data, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/images/generations", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call openai image generation: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai image generation returned %d: %s", resp.StatusCode, string(respBody))
	}

	var decoded struct {
		Data []struct {
			URL           string `json:"url"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode openai image response: %w", err)
	}
	if len(decoded.Data) == 0 {
		return nil, fmt.Errorf("openai image generation returned no images")
	}

	output := mustJSON(map[string]any{
		"images": []map[string]any{
			{
				"url":            decoded.Data[0].URL,
				"revised_prompt": decoded.Data[0].RevisedPrompt,
			},
		},
	})
	return &ImageResult{
		Output:      output,
		CostCredits: estimateImageCostCredits(p.imageModel, p.imageSize),
		Model:       p.imageModel,
	}, nil
}

func estimateImageCostCredits(model, size string) int {
	switch {
	case strings.Contains(model, "dall-e-3"):
		switch size {
		case "1024x1024":
			return 4
		case "1024x1792", "1792x1024":
			return 8
		default:
			return 8
		}
	case strings.Contains(model, "dall-e-2"):
		switch size {
		case "1024x1024":
			return 2
		case "512x512":
			return 1
		default:
			return 2
		}
	default:
		return 5
	}
}

func estimateTextCostCredits(totalTokens int) int {
	if totalTokens <= 0 {
		return 1
	}
	credits := totalTokens / 1000
	if totalTokens%1000 != 0 {
		credits++
	}
	if credits < 1 {
		return 1
	}
	return credits
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
