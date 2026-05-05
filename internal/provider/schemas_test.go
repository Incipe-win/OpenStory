package provider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestStructuredFallbacksValidate(t *testing.T) {
	for _, taskType := range []string{TaskIdea, TaskScript, TaskCharacter, TaskStoryboard, TaskImagePrompt, TaskVideoPrompt, TaskCreativePipeline} {
		spec := SchemaForTask(taskType)
		output := StructuredFallback(taskType, json.RawMessage(`{"prompt":"a drone flies through a canyon"}`))
		if err := ValidateStructuredJSON(spec.Schema, output); err != nil {
			t.Fatalf("%s fallback failed validation: %v\n%s", taskType, err, output)
		}
	}
}

func TestExecuteStructuredTextRetriesThenFallback(t *testing.T) {
	p := &invalidTextProvider{}
	req := Request{
		TaskType: TaskScript,
		Provider: p.Name(),
		Input:    json.RawMessage(`{"prompt":"test"}`),
	}

	result, err := Execute(context.Background(), p, req, 2)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if p.calls != 2 {
		t.Fatalf("expected 2 provider attempts, got %d", p.calls)
	}
	if result.CostCredits != 0 {
		t.Fatalf("fallback should have zero provider cost, got %d", result.CostCredits)
	}
	if err := ValidateStructuredJSON(SchemaForTask(TaskScript).Schema, result.Output); err != nil {
		t.Fatalf("fallback result invalid: %v", err)
	}
}

func TestExecuteReturnsUnsupportedCapability(t *testing.T) {
	_, err := Execute(context.Background(), &textOnlyProvider{}, Request{TaskType: TaskImageGeneration}, 1)
	if !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("expected ErrUnsupportedCapability, got %v", err)
	}
}

type invalidTextProvider struct {
	calls int
}

func (p *invalidTextProvider) Name() string { return "invalid-text" }
func (p *invalidTextProvider) Capabilities() []Capability {
	return []Capability{CapabilityTextGenerate}
}
func (p *invalidTextProvider) TextGenerate(context.Context, TextRequest) (*TextResult, error) {
	p.calls++
	return &TextResult{Output: json.RawMessage(`{"bad":true}`), CostCredits: 1, Model: "invalid"}, nil
}

type textOnlyProvider struct{}

func (textOnlyProvider) Name() string { return "text-only" }
func (textOnlyProvider) Capabilities() []Capability {
	return []Capability{CapabilityTextGenerate}
}
func (textOnlyProvider) TextGenerate(context.Context, TextRequest) (*TextResult, error) {
	return &TextResult{Output: StructuredFallback(TaskScript, nil)}, nil
}
