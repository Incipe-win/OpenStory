// Package provider implements the model provider gateway.
package provider

import (
	"context"
	"encoding/json"
	"errors"
)

type Capability string

const (
	CapabilityTextGenerate  Capability = "text_generate"
	CapabilityImageGenerate Capability = "image_generate"
	CapabilityVideoGenerate Capability = "video_generate"
	CapabilityEmbedding     Capability = "embedding"
)

var (
	ErrProviderNotConfigured = errors.New("provider not configured")
	ErrUnsupportedCapability = errors.New("provider does not support requested capability")
	ErrSchemaValidation      = errors.New("provider output failed JSON schema validation")
)

// Request is the normalized generation request used by task workers.
type Request struct {
	TaskType string          `json:"task_type"`
	Provider string          `json:"provider"`
	Input    json.RawMessage `json:"input"`
}

// Usage captures token or unit usage returned by a provider.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// Result is the normalized generation result persisted in generation_tasks.
type Result struct {
	Output      json.RawMessage `json:"output"`
	CostCredits int             `json:"cost_credits"`
	Capability  Capability      `json:"capability"`
	Model       string          `json:"model,omitempty"`
	Usage       Usage           `json:"usage,omitempty"`
}

type Provider interface {
	Name() string
	Capabilities() []Capability
}

type TextGenerator interface {
	Provider
	TextGenerate(ctx context.Context, req TextRequest) (*TextResult, error)
}

type ImageGenerator interface {
	Provider
	ImageGenerate(ctx context.Context, req ImageRequest) (*ImageResult, error)
}

type VideoGenerator interface {
	Provider
	VideoGenerate(ctx context.Context, req VideoRequest) (*VideoResult, error)
}

// EmbeddingGenerator is reserved for future RAG/search phases.
type EmbeddingGenerator interface {
	Provider
	Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResult, error)
}

type TextRequest struct {
	TaskType     string          `json:"task_type"`
	SystemPrompt string          `json:"system_prompt"`
	UserPrompt   string          `json:"user_prompt"`
	Input        json.RawMessage `json:"input"`
	SchemaName   string          `json:"schema_name"`
	Schema       json.RawMessage `json:"schema"`
	MaxAttempts  int             `json:"max_attempts"`
}

type TextResult struct {
	Output      json.RawMessage `json:"output"`
	CostCredits int             `json:"cost_credits"`
	Model       string          `json:"model,omitempty"`
	Usage       Usage           `json:"usage,omitempty"`
}

type ImageRequest struct {
	TaskType string          `json:"task_type"`
	Input    json.RawMessage `json:"input"`
	Prompt   string          `json:"prompt"`
	Model    string          `json:"model,omitempty"`
}

type ImageResult struct {
	Output      json.RawMessage `json:"output"`
	CostCredits int             `json:"cost_credits"`
	Model       string          `json:"model,omitempty"`
}

type VideoRequest struct {
	TaskType string          `json:"task_type"`
	Input    json.RawMessage `json:"input"`
	Prompt   string          `json:"prompt"`
	ImageURL string          `json:"image_url,omitempty"`
	Model    string          `json:"model,omitempty"`
}

type VideoResult struct {
	Output      json.RawMessage `json:"output"`
	CostCredits int             `json:"cost_credits"`
	Model       string          `json:"model,omitempty"`
}

type EmbeddingRequest struct {
	Input json.RawMessage `json:"input"`
	Model string          `json:"model,omitempty"`
}

type EmbeddingResult struct {
	Vectors [][]float32 `json:"vectors"`
	Model   string      `json:"model,omitempty"`
	Usage   Usage       `json:"usage,omitempty"`
}

type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

func (r *Registry) GetText(name string) (TextGenerator, bool) {
	p, ok := r.Get(name)
	if !ok {
		return nil, false
	}
	t, ok := p.(TextGenerator)
	return t, ok
}

func (r *Registry) GetImage(name string) (ImageGenerator, bool) {
	p, ok := r.Get(name)
	if !ok {
		return nil, false
	}
	img, ok := p.(ImageGenerator)
	return img, ok
}

func (r *Registry) GetVideo(name string) (VideoGenerator, bool) {
	p, ok := r.Get(name)
	if !ok {
		return nil, false
	}
	video, ok := p.(VideoGenerator)
	return video, ok
}

func HasCapability(p Provider, capability Capability) bool {
	for _, c := range p.Capabilities() {
		if c == capability {
			return true
		}
	}
	return false
}
