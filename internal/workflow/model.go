package workflow

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ── Node Types ──────────────────────────────────────

// NodeType defines the type of a workflow node.
type NodeType string

const (
	NodeIdea            NodeType = "idea"
	NodeScript          NodeType = "script"
	NodeCharacter       NodeType = "character"
	NodeScene           NodeType = "scene"
	NodeStoryboard      NodeType = "storyboard"
	NodeImagePrompt     NodeType = "image_prompt"
	NodeImageGeneration NodeType = "image_generation"
	NodeVideoGeneration NodeType = "video_generation"
	NodeAudio           NodeType = "audio"
	NodeSubtitle        NodeType = "subtitle"
	NodeCompose         NodeType = "compose"
)

// AllNodeTypes lists all valid node types.
var AllNodeTypes = []NodeType{
	NodeIdea, NodeScript, NodeCharacter, NodeScene, NodeStoryboard,
	NodeImagePrompt, NodeImageGeneration, NodeVideoGeneration,
	NodeAudio, NodeSubtitle, NodeCompose,
}

// ValidNodeType checks if a string is a valid node type.
func ValidNodeType(t string) bool {
	for _, nt := range AllNodeTypes {
		if string(nt) == t {
			return true
		}
	}
	return false
}

// ── Models ──────────────────────────────────────────

const (
	StatusDraft     = "draft"
	StatusValidated = "validated"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Workflow represents a DAG workflow for a project.
type Workflow struct {
	ID             uuid.UUID `json:"id"`
	ProjectID      uuid.UUID `json:"project_id"`
	UserID         uuid.UUID `json:"user_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
	CurrentVersion int       `json:"current_version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Node represents a single node in the workflow DAG.
type Node struct {
	ID         uuid.UUID       `json:"id"`
	WorkflowID uuid.UUID       `json:"workflow_id"`
	Type       NodeType        `json:"type"`
	Name       string          `json:"name"`
	ConfigJSON json.RawMessage `json:"config"`
	PositionX  float64         `json:"position_x"`
	PositionY  float64         `json:"position_y"`
	Status     string          `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// Edge represents a connection between two nodes.
type Edge struct {
	ID           uuid.UUID `json:"id"`
	WorkflowID   uuid.UUID `json:"workflow_id"`
	SourceNodeID uuid.UUID `json:"source_node_id"`
	TargetNodeID uuid.UUID `json:"target_node_id"`
	SourceHandle string    `json:"source_handle"`
	TargetHandle string    `json:"target_handle"`
	CreatedAt    time.Time `json:"created_at"`
}

// Version represents a versioned snapshot of a workflow.
type Version struct {
	ID           uuid.UUID       `json:"id"`
	WorkflowID   uuid.UUID       `json:"workflow_id"`
	VersionNum   int             `json:"version"`
	SnapshotJSON json.RawMessage `json:"snapshot"`
	CreatedBy    uuid.UUID       `json:"created_by"`
	CreatedAt    time.Time       `json:"created_at"`
}

// WorkflowDetail is the full workflow with nodes and edges.
type WorkflowDetail struct {
	Workflow
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Snapshot captures the full state for versioning.
type Snapshot struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}
