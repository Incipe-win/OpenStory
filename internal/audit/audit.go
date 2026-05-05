// Package audit provides audit logging for critical operations.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Logger writes audit log entries to the database.
type Logger struct {
	pool *pgxpool.Pool
}

// NewLogger creates a new audit logger.
func NewLogger(pool *pgxpool.Pool) *Logger {
	return &Logger{pool: pool}
}

// Entry represents an audit log entry.
type Entry struct {
	UserID       *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	IPAddress    string
	UserAgent    string
	OldValues    any
	NewValues    any
	TraceID      string
}

// Log writes an audit entry to the database.
func (l *Logger) Log(ctx context.Context, e Entry) error {
	oldJSON, _ := json.Marshal(e.OldValues)
	newJSON, _ := json.Marshal(e.NewValues)

	_, err := l.pool.Exec(ctx,
		`INSERT INTO audit_logs (user_id, action, resource_type, resource_id, ip_address, user_agent, old_values_json, new_values_json, trace_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		e.UserID, e.Action, e.ResourceType, e.ResourceID, e.IPAddress, e.UserAgent, oldJSON, newJSON, e.TraceID,
	)
	if err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}
	return nil
}
