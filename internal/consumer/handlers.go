package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
)

func Definitions() []Definition {
	return []Definition{
		{
			Name:    NameAnalytics,
			GroupID: NameAnalytics,
			Topic:   eventbus.TopicGenerationTaskEvents,
			Handler: HandlerFunc(handleAnalytics),
		},
		{
			Name:    NameNotification,
			GroupID: NameNotification,
			Topic:   eventbus.TopicGenerationTaskEvents,
			Handler: HandlerFunc(handleNotification),
		},
		{
			Name:    NameFeed,
			GroupID: NameFeed,
			Topic:   eventbus.TopicWorkEvents,
			Handler: HandlerFunc(handleFeed),
		},
		{
			Name:    NameModeration,
			GroupID: NameModeration,
			Topic:   eventbus.TopicWorkEvents,
			Handler: HandlerFunc(handleModeration),
		},
		{
			Name:    NameAudit,
			GroupID: NameAudit,
			Topic:   eventbus.TopicAuditEvents,
			Handler: HandlerFunc(handleAudit),
		},
	}
}

func DefinitionByName(name string) (Definition, bool) {
	for _, def := range Definitions() {
		if def.Name == name {
			return def, true
		}
	}
	return Definition{}, false
}

func handleAnalytics(ctx context.Context, tx pgx.Tx, event eventbus.Event) error {
	if event.EventType != "task_succeeded" && event.EventType != "task_failed" && event.EventType != "task_canceled" {
		return nil
	}

	payload := payloadMap(event)
	provider := stringPayload(payload, "provider", "unknown")
	taskType := stringPayload(payload, "task_type", "unknown")
	status := stringPayload(payload, "status", event.EventType)
	cost := intPayload(payload, "cost_credits")
	duration := int64(0)

	if event.EventType == "task_succeeded" {
		_ = tx.QueryRow(ctx,
			`SELECT COALESCE(EXTRACT(EPOCH FROM (finished_at - started_at)) * 1000, 0)::BIGINT
			 FROM generation_tasks WHERE id = $1`,
			event.AggregateID,
		).Scan(&duration)
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO analytics_task_metrics
			(bucket_date, provider, task_type, total_tasks, succeeded_tasks, failed_tasks, canceled_tasks, total_duration_ms, total_cost_credits)
		 VALUES (CURRENT_DATE, $1, $2, 1,
			CASE WHEN $3 = 'succeeded' THEN 1 ELSE 0 END,
			CASE WHEN $3 = 'failed' THEN 1 ELSE 0 END,
			CASE WHEN $3 = 'canceled' THEN 1 ELSE 0 END,
			$4, $5)
		 ON CONFLICT (bucket_date, provider, task_type)
		 DO UPDATE SET
			total_tasks = analytics_task_metrics.total_tasks + 1,
			succeeded_tasks = analytics_task_metrics.succeeded_tasks + CASE WHEN $3 = 'succeeded' THEN 1 ELSE 0 END,
			failed_tasks = analytics_task_metrics.failed_tasks + CASE WHEN $3 = 'failed' THEN 1 ELSE 0 END,
			canceled_tasks = analytics_task_metrics.canceled_tasks + CASE WHEN $3 = 'canceled' THEN 1 ELSE 0 END,
			total_duration_ms = analytics_task_metrics.total_duration_ms + $4,
			total_cost_credits = analytics_task_metrics.total_cost_credits + $5,
			updated_at = NOW()`,
		provider, taskType, normalizeTaskStatus(status, event.EventType), duration, cost,
	)
	if err != nil {
		return fmt.Errorf("update analytics task metrics: %w", err)
	}
	return nil
}

func handleNotification(ctx context.Context, tx pgx.Tx, event eventbus.Event) error {
	if event.EventType != "task_succeeded" && event.EventType != "task_failed" && event.EventType != "task_canceled" {
		return nil
	}
	if event.UserID == uuid.Nil {
		return nil
	}

	title := "Generation task completed"
	if event.EventType == "task_failed" {
		title = "Generation task failed"
	}
	if event.EventType == "task_canceled" {
		title = "Generation task canceled"
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO notifications (event_id, user_id, type, title, body)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (event_id) DO NOTHING`,
		event.EventID, event.UserID, event.EventType, title,
		fmt.Sprintf("Task %s reached %s", event.AggregateID, strings.TrimPrefix(event.EventType, "task_")),
	)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

func handleFeed(ctx context.Context, tx pgx.Tx, event eventbus.Event) error {
	if event.EventType != "work_published" {
		return nil
	}

	payload := payloadMap(event)
	projectID, err := uuid.Parse(stringPayload(payload, "project_id", ""))
	if err != nil {
		return fmt.Errorf("parse project_id: %w", err)
	}

	metadata, _ := json.Marshal(payload)
	_, err = tx.Exec(ctx,
		`INSERT INTO feed_items (work_id, event_id, user_id, project_id, title, metadata_json, published_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (work_id)
		 DO UPDATE SET
			event_id = EXCLUDED.event_id,
			title = EXCLUDED.title,
			metadata_json = EXCLUDED.metadata_json,
			published_at = EXCLUDED.published_at,
			updated_at = NOW()`,
		event.AggregateID, event.EventID, event.UserID, projectID, stringPayload(payload, "title", "Untitled"), metadata,
	)
	if err != nil {
		return fmt.Errorf("upsert feed item: %w", err)
	}
	return nil
}

func handleModeration(ctx context.Context, tx pgx.Tx, event eventbus.Event) error {
	if event.EventType != "work_published" || event.UserID == uuid.Nil {
		return nil
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO moderation_records (user_id, target_type, target_id, provider, status, categories_json)
		 VALUES ($1, 'work', $2, 'internal', 'pending', '{}')
		 ON CONFLICT (target_type, target_id) DO NOTHING`,
		event.UserID, event.AggregateID,
	)
	if err != nil {
		return fmt.Errorf("enqueue moderation record: %w", err)
	}
	return nil
}

func handleAudit(ctx context.Context, tx pgx.Tx, event eventbus.Event) error {
	payload, _ := json.Marshal(event.Payload)
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (user_id, action, resource_type, resource_id, ip_address, user_agent, new_values_json, trace_id)
		 VALUES ($1, $2, $3, $4, '', 'audit-consumer', $5, $6)`,
		uuidPtr(event.UserID), event.EventType, event.AggregateType, event.AggregateID, payload, event.TraceID,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func payloadMap(event eventbus.Event) map[string]any {
	if event.Payload == nil {
		return map[string]any{}
	}
	if payload, ok := event.Payload.(map[string]any); ok {
		return payload
	}
	data, _ := json.Marshal(event.Payload)
	var payload map[string]any
	_ = json.Unmarshal(data, &payload)
	return payload
}

func stringPayload(payload map[string]any, key, fallback string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case string:
		if v == "" {
			return fallback
		}
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func intPayload(payload map[string]any, key string) int {
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return 0
	}
}

func normalizeTaskStatus(status, eventType string) string {
	if status == "succeeded" || status == "failed" || status == "canceled" {
		return status
	}
	return strings.TrimPrefix(eventType, "task_")
}
