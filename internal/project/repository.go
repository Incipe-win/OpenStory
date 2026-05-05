package project

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrWorkNotFound    = errors.New("work not found")
)

// Repository defines project and work database operations.
type Repository interface {
	CreateProject(ctx context.Context, p *Project) error
	GetProject(ctx context.Context, id uuid.UUID) (*Project, error)
	ListProjects(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]Project, int, error)
	UpdateProject(ctx context.Context, id, userID uuid.UUID, name, description *string, status *string) error
	GetWork(ctx context.Context, id uuid.UUID) (*Work, error)
	PublishWork(ctx context.Context, id, userID uuid.UUID) error
	ListPublishedWorks(ctx context.Context, page, pageSize int) ([]Work, int, error)
}

// PgRepository implements Repository using pgxpool.
type PgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

func (r *PgRepository) CreateProject(ctx context.Context, p *Project) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO projects (user_id, name, description)
		 VALUES ($1, $2, $3)
		 RETURNING id, status, settings_json, created_at, updated_at`,
		p.UserID, p.Name, p.Description,
	).Scan(&p.ID, &p.Status, &p.SettingsJSON, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("inserting project: %w", err)
	}
	return nil
}

func (r *PgRepository) GetProject(ctx context.Context, id uuid.UUID) (*Project, error) {
	p := &Project{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, name, description, cover_url, status, settings_json, created_at, updated_at
		 FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.CoverURL,
		&p.Status, &p.SettingsJSON, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying project: %w", err)
	}
	return p, nil
}

func (r *PgRepository) ListProjects(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]Project, int, error) {
	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM projects WHERE user_id = $1 AND status != 'deleted'`, userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting projects: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, name, description, cover_url, status, settings_json, created_at, updated_at
		 FROM projects WHERE user_id = $1 AND status != 'deleted'
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.CoverURL,
			&p.Status, &p.SettingsJSON, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, total, nil
}

func (r *PgRepository) UpdateProject(ctx context.Context, id, userID uuid.UUID, name, description, status *string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE projects SET
			name        = COALESCE($3, name),
			description = COALESCE($4, description),
			status      = COALESCE($5, status)
		 WHERE id = $1 AND user_id = $2`,
		id, userID, name, description, status,
	)
	if err != nil {
		return fmt.Errorf("updating project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrProjectNotFound
	}
	return nil
}

func (r *PgRepository) GetWork(ctx context.Context, id uuid.UUID) (*Work, error) {
	w := &Work{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, project_id, user_id, title, description, duration_ms, resolution, format,
		        file_url, thumbnail_url, status, metadata_json, published_at, created_at, updated_at
		 FROM works WHERE id = $1`, id,
	).Scan(&w.ID, &w.ProjectID, &w.UserID, &w.Title, &w.Description,
		&w.DurationMs, &w.Resolution, &w.Format, &w.FileURL, &w.ThumbnailURL,
		&w.Status, &w.MetadataJSON, &w.PublishedAt, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWorkNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying work: %w", err)
	}
	return w, nil
}

func (r *PgRepository) PublishWork(ctx context.Context, id, userID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin publish work tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var ownerID uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT user_id FROM works WHERE id = $1 AND user_id = $2 FOR UPDATE`,
		id, userID,
	).Scan(&ownerID); errors.Is(err, pgx.ErrNoRows) {
		return ErrWorkNotFound
	} else if err != nil {
		return fmt.Errorf("query work for review submission: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE works SET status = 'pending_review', published_at = NULL
		 WHERE id = $1 AND user_id = $2`,
		id, userID,
	); err != nil {
		return fmt.Errorf("marking work pending review: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO moderation_records (user_id, target_type, target_id, provider, status, categories_json)
		 VALUES ($1, 'work', $2, 'internal', 'pending', '{}')
		 ON CONFLICT (target_type, target_id)
		 DO UPDATE SET status = 'pending', reason = NULL, reviewed_by = NULL, reviewed_at = NULL`,
		userID, id,
	); err != nil {
		return fmt.Errorf("upsert moderation record: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit publish work tx: %w", err)
	}
	return nil
}

func (r *PgRepository) ListPublishedWorks(ctx context.Context, page, pageSize int) ([]Work, int, error) {
	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM works WHERE status = 'published'`,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting works: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx,
		`SELECT id, project_id, user_id, title, description, duration_ms, resolution, format,
		        file_url, thumbnail_url, status, metadata_json, published_at, created_at, updated_at
		 FROM works WHERE status = 'published'
		 ORDER BY published_at DESC LIMIT $1 OFFSET $2`,
		pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing published works: %w", err)
	}
	defer rows.Close()

	var works []Work
	for rows.Next() {
		var w Work
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.UserID, &w.Title, &w.Description,
			&w.DurationMs, &w.Resolution, &w.Format, &w.FileURL, &w.ThumbnailURL,
			&w.Status, &w.MetadataJSON, &w.PublishedAt, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning work: %w", err)
		}
		works = append(works, w)
	}
	return works, total, nil
}
