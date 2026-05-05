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

type OpenAITextProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewOpenAITextProvider(baseURL, apiKey, model string) *OpenAITextProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAITextProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OpenAITextProvider) Name() string { return "openai-compatible" }

func (p *OpenAITextProvider) Capabilities() []Capability {
	return []Capability{CapabilityTextGenerate}
}

func (p *OpenAITextProvider) TextGenerate(ctx context.Context, req TextRequest) (*TextResult, error) {
	if p.apiKey == "" {
		return nil, ErrProviderNotConfigured
	}

	var schema map[string]any
	if err := json.Unmarshal(req.Schema, &schema); err != nil {
		return nil, fmt.Errorf("decode schema for provider request: %w", err)
	}

	body := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"temperature": 0.2,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   req.SchemaName,
				"strict": true,
				"schema": schema,
			},
		},
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
