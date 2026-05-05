package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrWorkflowNotFound = errors.New("workflow not found")
)

// Repository defines workflow database operations.
type Repository interface {
	Create(ctx context.Context, wf *Workflow) error
	GetDetail(ctx context.Context, id uuid.UUID) (*WorkflowDetail, error)
	Update(ctx context.Context, wf *Workflow, nodes []Node, edges []Edge) error
	CreateSnapshot(ctx context.Context, workflowID, userID uuid.UUID) (*Version, error)
}

// PgRepository implements Repository using pgxpool.
type PgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

func (r *PgRepository) Create(ctx context.Context, wf *Workflow) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO workflows (project_id, user_id, name, description)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, status, current_version, created_at, updated_at`,
		wf.ProjectID, wf.UserID, wf.Name, wf.Description,
	).Scan(&wf.ID, &wf.Status, &wf.CurrentVersion, &wf.CreatedAt, &wf.UpdatedAt)
}

func (r *PgRepository) GetDetail(ctx context.Context, id uuid.UUID) (*WorkflowDetail, error) {
	d := &WorkflowDetail{}

	// Get workflow
	err := r.pool.QueryRow(ctx,
		`SELECT id, project_id, user_id, name, description, status, current_version, created_at, updated_at
		 FROM workflows WHERE id = $1`, id,
	).Scan(&d.ID, &d.ProjectID, &d.UserID, &d.Name, &d.Description,
		&d.Status, &d.CurrentVersion, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWorkflowNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get workflow: %w", err)
	}

	// Get nodes
	rows, err := r.pool.Query(ctx,
		`SELECT id, workflow_id, type, name, config_json, position_x, position_y, status, created_at, updated_at
		 FROM workflow_nodes WHERE workflow_id = $1 ORDER BY created_at`, id)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.WorkflowID, &n.Type, &n.Name, &n.ConfigJSON,
			&n.PositionX, &n.PositionY, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		d.Nodes = append(d.Nodes, n)
	}

	// Get edges
	rows2, err := r.pool.Query(ctx,
		`SELECT id, workflow_id, source_node_id, target_node_id, source_handle, target_handle, created_at
		 FROM workflow_edges WHERE workflow_id = $1 ORDER BY created_at`, id)
	if err != nil {
		return nil, fmt.Errorf("list edges: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var e Edge
		if err := rows2.Scan(&e.ID, &e.WorkflowID, &e.SourceNodeID, &e.TargetNodeID,
			&e.SourceHandle, &e.TargetHandle, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		d.Edges = append(d.Edges, e)
	}

	if d.Nodes == nil {
		d.Nodes = []Node{}
	}
	if d.Edges == nil {
		d.Edges = []Edge{}
	}

	return d, nil
}

func (r *PgRepository) Update(ctx context.Context, wf *Workflow, nodes []Node, edges []Edge) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update workflow metadata
	tag, err := tx.Exec(ctx,
		`UPDATE workflows SET name = $2, description = $3, status = $4 WHERE id = $1`,
		wf.ID, wf.Name, wf.Description, wf.Status)
	if err != nil {
		return fmt.Errorf("update workflow: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrWorkflowNotFound
	}

	// Delete existing nodes and edges (cascade handles edges via FK)
	if _, err := tx.Exec(ctx, `DELETE FROM workflow_nodes WHERE workflow_id = $1`, wf.ID); err != nil {
		return fmt.Errorf("delete old nodes: %w", err)
	}

	// Insert new nodes
	for i := range nodes {
		n := &nodes[i]
		n.WorkflowID = wf.ID
		if n.ConfigJSON == nil {
			n.ConfigJSON = json.RawMessage(`{}`)
		}
		err := tx.QueryRow(ctx,
			`INSERT INTO workflow_nodes (id, workflow_id, type, name, config_json, position_x, position_y)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING status, created_at, updated_at`,
			n.ID, n.WorkflowID, n.Type, n.Name, n.ConfigJSON, n.PositionX, n.PositionY,
		).Scan(&n.Status, &n.CreatedAt, &n.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert node %s: %w", n.ID, err)
		}
	}

	// Insert new edges
	for i := range edges {
		e := &edges[i]
		e.WorkflowID = wf.ID
		if e.ID == uuid.Nil {
			e.ID = uuid.New()
		}
		if e.SourceHandle == "" {
			e.SourceHandle = "default"
		}
		if e.TargetHandle == "" {
			e.TargetHandle = "default"
		}
		err := tx.QueryRow(ctx,
			`INSERT INTO workflow_edges (id, workflow_id, source_node_id, target_node_id, source_handle, target_handle)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING created_at`,
			e.ID, e.WorkflowID, e.SourceNodeID, e.TargetNodeID, e.SourceHandle, e.TargetHandle,
		).Scan(&e.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert edge: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *PgRepository) CreateSnapshot(ctx context.Context, workflowID, userID uuid.UUID) (*Version, error) {
	detail, err := r.GetDetail(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	snapshot := Snapshot{Nodes: detail.Nodes, Edges: detail.Edges}
	snapJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Increment version
	newVersion := detail.CurrentVersion + 1
	if _, err := tx.Exec(ctx,
		`UPDATE workflows SET current_version = $2 WHERE id = $1`,
		workflowID, newVersion); err != nil {
		return nil, fmt.Errorf("update version: %w", err)
	}

	v := &Version{
		WorkflowID:   workflowID,
		VersionNum:   newVersion,
		SnapshotJSON: snapJSON,
		CreatedBy:    userID,
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO workflow_versions (workflow_id, version, snapshot_json, created_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		v.WorkflowID, v.VersionNum, v.SnapshotJSON, v.CreatedBy,
	).Scan(&v.ID, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	return v, tx.Commit(ctx)
}
