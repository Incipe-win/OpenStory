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

type ReplicateProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewReplicateProvider(baseURL, apiKey string) *ReplicateProvider {
	if baseURL == "" {
		baseURL = "https://api.replicate.com/v1"
	}
	return &ReplicateProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *ReplicateProvider) Name() string { return "replicate" }

func (p *ReplicateProvider) Capabilities() []Capability {
	return []Capability{CapabilityImageGenerate, CapabilityVideoGenerate}
}

func (p *ReplicateProvider) ImageGenerate(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	output, model, err := p.createPrediction(ctx, req.Model, map[string]any{
		"prompt": req.Prompt,
		"input":  json.RawMessage(req.Input),
	})
	if err != nil {
		return nil, err
	}
	return &ImageResult{Output: output, CostCredits: 8, Model: model}, nil
}

func (p *ReplicateProvider) VideoGenerate(ctx context.Context, req VideoRequest) (*VideoResult, error) {
	input := map[string]any{
		"prompt": req.Prompt,
		"input":  json.RawMessage(req.Input),
	}
	if req.ImageURL != "" {
		input["image"] = req.ImageURL
	}
	output, model, err := p.createPrediction(ctx, req.Model, input)
	if err != nil {
		return nil, err
	}
	return &VideoResult{Output: output, CostCredits: 12, Model: model}, nil
}

func (p *ReplicateProvider) createPrediction(ctx context.Context, model string, input map[string]any) (json.RawMessage, string, error) {
	if p.apiKey == "" {
		return nil, "", ErrProviderNotConfigured
	}
	if model == "" {
		return nil, "", fmt.Errorf("replicate model/version is required")
	}
	body := map[string]any{
		"version": model,
		"input":   input,
	}
	data, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/predictions", bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Token "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("call replicate adapter: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("replicate adapter returned %d: %s", resp.StatusCode, string(respBody))
	}
	var decoded map[string]any
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, "", fmt.Errorf("decode replicate response: %w", err)
	}
	output := mustJSON(map[string]any{
		"provider":      "replicate",
		"status":        decoded["status"],
		"prediction_id": decoded["id"],
		"urls":          decoded["urls"],
	})
	return output, model, nil
}
