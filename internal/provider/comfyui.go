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

type ComfyUIProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewComfyUIProvider(baseURL, apiKey string) *ComfyUIProvider {
	return &ComfyUIProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *ComfyUIProvider) Name() string { return "comfyui" }

func (p *ComfyUIProvider) Capabilities() []Capability {
	return []Capability{CapabilityImageGenerate, CapabilityVideoGenerate}
}

func (p *ComfyUIProvider) ImageGenerate(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	return p.enqueue(ctx, "image", req.Prompt, req.Input, req.Model, 8)
}

func (p *ComfyUIProvider) VideoGenerate(ctx context.Context, req VideoRequest) (*VideoResult, error) {
	result, err := p.enqueue(ctx, "video", req.Prompt, req.Input, req.Model, 12)
	if err != nil {
		return nil, err
	}
	return &VideoResult{Output: result.Output, CostCredits: result.CostCredits, Model: result.Model}, nil
}

func (p *ComfyUIProvider) enqueue(ctx context.Context, kind, prompt string, input json.RawMessage, model string, cost int) (*ImageResult, error) {
	if p.baseURL == "" {
		return nil, ErrProviderNotConfigured
	}
	body := map[string]any{
		"client_id": "openstory",
		"prompt": map[string]any{
			"kind":   kind,
			"prompt": prompt,
			"input":  json.RawMessage(input),
			"model":  model,
		},
	}
	data, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/prompt", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call comfyui adapter: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("comfyui adapter returned %d: %s", resp.StatusCode, string(respBody))
	}
	var decoded map[string]any
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode comfyui response: %w", err)
	}
	output := mustJSON(map[string]any{
		"provider":  "comfyui",
		"status":    "queued",
		"prompt_id": decoded["prompt_id"],
		"kind":      kind,
	})
	return &ImageResult{Output: output, CostCredits: cost, Model: firstNonEmpty(model, "comfyui-workflow")}, nil
}
