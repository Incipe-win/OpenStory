// Package provider implements AI model provider gateway.
package provider

import (
	"context"
	"encoding/json"
)

// Request represents an AI generation request.
type Request struct {
	TaskType string          `json:"task_type"`
	Provider string          `json:"provider"`
	Input    json.RawMessage `json:"input"`
}

// Result represents an AI generation result.
type Result struct {
	Output      json.RawMessage `json:"output"`
	CostCredits int             `json:"cost_credits"`
}

// Provider defines the interface for AI model providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string
	// Generate executes an AI generation task.
	Generate(ctx context.Context, req Request) (*Result, error)
	// Supports returns true if this provider handles the given task type.
	Supports(taskType string) bool
}

// Registry holds registered providers and dispatches by name.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

// Get retrieves a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}
