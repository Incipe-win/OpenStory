package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	TaskIdea             = "idea"
	TaskScript           = "script"
	TaskCharacter        = "character"
	TaskCharacters       = "characters"
	TaskScene            = "scene"
	TaskStoryboard       = "storyboard"
	TaskImagePrompt      = "image_prompt"
	TaskVideoPrompt      = "video_prompt"
	TaskCreativePipeline = "creative_pipeline"
	TaskImageGeneration  = "image_generation"
	TaskVideoGeneration  = "video_generation"
)

type SchemaSpec struct {
	Name   string
	Schema json.RawMessage
}

func SchemaForTask(taskType string) SchemaSpec {
	switch normalizeTaskType(taskType) {
	case TaskIdea:
		return schemaSpec("idea_brief", ideaSchema)
	case TaskScript:
		return schemaSpec("script", scriptSchema)
	case TaskCharacter, TaskCharacters:
		return schemaSpec("characters", charactersSchema)
	case TaskScene:
		return schemaSpec("scenes", scenesSchema)
	case TaskStoryboard:
		return schemaSpec("storyboard", storyboardSchema)
	case TaskImagePrompt:
		return schemaSpec("image_prompts", imagePromptsSchema)
	case TaskVideoPrompt:
		return schemaSpec("video_prompts", videoPromptsSchema)
	case TaskCreativePipeline:
		return schemaSpec("creative_pipeline", creativePipelineSchema)
	default:
		return schemaSpec("generic_json", genericObjectSchema)
	}
}

func ValidateStructuredJSON(schema json.RawMessage, output json.RawMessage) error {
	if len(schema) == 0 || len(output) == 0 {
		return fmt.Errorf("%w: schema and output are required", ErrSchemaValidation)
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return fmt.Errorf("decode json schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", schemaDoc); err != nil {
		return fmt.Errorf("add json schema resource: %w", err)
	}
	compiled, err := c.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("compile json schema: %w", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(output))
	if err != nil {
		return fmt.Errorf("%w: invalid JSON: %v", ErrSchemaValidation, err)
	}
	if err := compiled.Validate(instance); err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaValidation, err)
	}
	return nil
}

func BuildTextRequest(req Request, maxAttempts int) TextRequest {
	spec := SchemaForTask(req.TaskType)
	return TextRequest{
		TaskType:     normalizeTaskType(req.TaskType),
		SystemPrompt: structuredSystemPrompt,
		UserPrompt:   buildUserPrompt(req.TaskType, req.Input),
		Input:        req.Input,
		SchemaName:   spec.Name,
		Schema:       spec.Schema,
		MaxAttempts:  maxAttempts,
	}
}

func StructuredFallback(taskType string, input json.RawMessage) json.RawMessage {
	switch normalizeTaskType(taskType) {
	case TaskIdea:
		return mustJSON(map[string]any{
			"idea":     promptFromInput(input),
			"theme":    "self-hosted AI video story",
			"tone":     "cinematic",
			"audience": "general",
		})
	case TaskScript:
		return mustJSON(map[string]any{
			"title":            "Untitled Story",
			"logline":          promptFromInput(input),
			"duration_seconds": 30,
			"scenes": []map[string]any{
				{"index": 1, "setting": "opening", "action": "Introduce the idea visually.", "dialogue": ""},
				{"index": 2, "setting": "middle", "action": "Develop the key conflict.", "dialogue": ""},
				{"index": 3, "setting": "ending", "action": "Resolve with a clear final image.", "dialogue": ""},
			},
		})
	case TaskCharacter, TaskCharacters:
		return mustJSON(map[string]any{
			"characters": []map[string]any{
				{"name": "Protagonist", "role": "lead", "visual_description": "expressive main character", "personality": "curious and determined"},
			},
		})
	case TaskStoryboard:
		return mustJSON(map[string]any{
			"shots": []map[string]any{
				{"index": 1, "scene_index": 1, "shot_type": "wide", "camera": "slow push in", "action": "Establish the world", "duration_seconds": 5},
				{"index": 2, "scene_index": 2, "shot_type": "medium", "camera": "tracking", "action": "Show the turning point", "duration_seconds": 5},
				{"index": 3, "scene_index": 3, "shot_type": "close-up", "camera": "locked", "action": "End on the key emotion", "duration_seconds": 5},
			},
		})
	case TaskImagePrompt:
		return mustJSON(map[string]any{
			"prompts": []map[string]any{
				{"shot_index": 1, "prompt": "cinematic establishing frame, detailed composition, natural light", "negative_prompt": "low quality, blurry"},
			},
		})
	case TaskVideoPrompt:
		return mustJSON(map[string]any{
			"prompts": []map[string]any{
				{"shot_index": 1, "prompt": "smooth cinematic camera movement, immersive motion, coherent subject", "duration_seconds": 5, "camera_fixed": false},
			},
		})
	case TaskCreativePipeline:
		return mustJSON(map[string]any{
			"script":        json.RawMessage(StructuredFallback(TaskScript, input)),
			"characters":    json.RawMessage(StructuredFallback(TaskCharacter, input)),
			"storyboard":    json.RawMessage(StructuredFallback(TaskStoryboard, input)),
			"image_prompts": json.RawMessage(StructuredFallback(TaskImagePrompt, input)),
			"video_prompts": json.RawMessage(StructuredFallback(TaskVideoPrompt, input)),
		})
	default:
		return mustJSON(map[string]any{"result": promptFromInput(input)})
	}
}

func buildUserPrompt(taskType string, input json.RawMessage) string {
	stage := normalizeTaskType(taskType)
	return fmt.Sprintf(`Generate the "%s" stage for an AI short-video workflow.

Workflow target:
idea -> script -> characters -> storyboard -> image prompts -> video prompts.

Input JSON:
%s

Return only JSON that validates against the provided schema. Do not include markdown or explanations.`, stage, string(input))
}

func normalizeTaskType(taskType string) string {
	switch strings.TrimSpace(taskType) {
	case TaskCharacters:
		return TaskCharacter
	default:
		return strings.TrimSpace(taskType)
	}
}

func schemaSpec(name, schema string) SchemaSpec {
	return SchemaSpec{Name: name, Schema: json.RawMessage(schema)}
}

func mustJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func promptFromInput(input json.RawMessage) string {
	var obj map[string]any
	if err := json.Unmarshal(input, &obj); err == nil {
		for _, key := range []string{"idea", "prompt", "text"} {
			if v, ok := obj[key].(string); ok && strings.TrimSpace(v) != "" {
				return v
			}
		}
	}
	if strings.TrimSpace(string(input)) == "" {
		return "Create a concise cinematic short video."
	}
	return strings.TrimSpace(string(input))
}

const structuredSystemPrompt = `You are OpenStory's structured creative engine. You only return strict JSON. No markdown, no code fences, no prose outside JSON.`

const genericObjectSchema = `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`

const ideaSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "required":["idea","theme","tone","audience"],
  "properties":{
    "idea":{"type":"string"},
    "theme":{"type":"string"},
    "tone":{"type":"string"},
    "audience":{"type":"string"}
  },
  "additionalProperties":false
}`

const scriptSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "required":["title","logline","duration_seconds","scenes"],
  "properties":{
    "title":{"type":"string"},
    "logline":{"type":"string"},
    "duration_seconds":{"type":"integer","minimum":1},
    "scenes":{
      "type":"array","minItems":1,
      "items":{"type":"object","required":["index","setting","action","dialogue"],"properties":{
        "index":{"type":"integer","minimum":1},
        "setting":{"type":"string"},
        "action":{"type":"string"},
        "dialogue":{"type":"string"}
      },"additionalProperties":false}
    }
  },
  "additionalProperties":false
}`

const charactersSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "required":["characters"],
  "properties":{
    "characters":{"type":"array","minItems":1,"items":{"type":"object","required":["name","role","visual_description","personality"],"properties":{
      "name":{"type":"string"},
      "role":{"type":"string"},
      "visual_description":{"type":"string"},
      "personality":{"type":"string"}
    },"additionalProperties":false}}
  },
  "additionalProperties":false
}`

const scenesSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "required":["scenes"],
  "properties":{
    "scenes":{"type":"array","minItems":1,"items":{"type":"object","required":["index","setting","summary","visual_mood"],"properties":{
      "index":{"type":"integer","minimum":1},
      "setting":{"type":"string"},
      "summary":{"type":"string"},
      "visual_mood":{"type":"string"}
    },"additionalProperties":false}}
  },
  "additionalProperties":false
}`

const storyboardSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "required":["shots"],
  "properties":{
    "shots":{"type":"array","minItems":1,"items":{"type":"object","required":["index","scene_index","shot_type","camera","action","duration_seconds"],"properties":{
      "index":{"type":"integer","minimum":1},
      "scene_index":{"type":"integer","minimum":1},
      "shot_type":{"type":"string"},
      "camera":{"type":"string"},
      "action":{"type":"string"},
      "duration_seconds":{"type":"integer","minimum":1}
    },"additionalProperties":false}}
  },
  "additionalProperties":false
}`

const imagePromptsSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "required":["prompts"],
  "properties":{
    "prompts":{"type":"array","minItems":1,"items":{"type":"object","required":["shot_index","prompt","negative_prompt"],"properties":{
      "shot_index":{"type":"integer","minimum":1},
      "prompt":{"type":"string"},
      "negative_prompt":{"type":"string"}
    },"additionalProperties":false}}
  },
  "additionalProperties":false
}`

const videoPromptsSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "required":["prompts"],
  "properties":{
    "prompts":{"type":"array","minItems":1,"items":{"type":"object","required":["shot_index","prompt","duration_seconds","camera_fixed"],"properties":{
      "shot_index":{"type":"integer","minimum":1},
      "prompt":{"type":"string"},
      "duration_seconds":{"type":"integer","minimum":1},
      "camera_fixed":{"type":"boolean"}
    },"additionalProperties":false}}
  },
  "additionalProperties":false
}`

const creativePipelineSchema = `{
  "$schema":"https://json-schema.org/draft/2020-12/schema",
  "type":"object",
  "required":["script","characters","storyboard","image_prompts","video_prompts"],
  "properties":{
    "script":` + scriptSchema + `,
    "characters":` + charactersSchema + `,
    "storyboard":` + storyboardSchema + `,
    "image_prompts":` + imagePromptsSchema + `,
    "video_prompts":` + videoPromptsSchema + `
  },
  "additionalProperties":false
}`
