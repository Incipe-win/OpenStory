// Package outbox implements the transactional outbox pattern relay.
// It polls the outbox_events table and publishes to Kafka, then marks published_at.
package outbox

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
)

// ── Prometheus Metrics ──────────────────────────────

var (
	outboxBacklog = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "openstory",
		Subsystem: "outbox",
		Name:      "backlog_total",
		Help:      "Number of unpublished outbox events",
	})
	outboxPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openstory",
		Subsystem: "outbox",
		Name:      "published_total",
		Help:      "Total outbox events published to Kafka",
	}, []string{"topic"})
	outboxFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openstory",
		Subsystem: "outbox",
		Name:      "failures_total",
		Help:      "Total outbox publishing failures",
	}, []string{"topic"})
	outboxPublishDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "openstory",
		Subsystem: "outbox",
		Name:      "publish_duration_seconds",
		Help:      "Kafka publish latency",
		Buckets:   prometheus.DefBuckets,
	}, []string{"topic"})
)

// ── Outbox Row ──────────────────────────────────────

type outboxRow struct {
	ID            int64
	EventType     string
	AggregateType string
	AggregateID   string
	PayloadJSON   []byte
	SchemaVersion int
	TraceID       string
	CreatedAt     time.Time
	RetryCount    int
}

// ── Relay ───────────────────────────────────────────

// Relay polls the outbox_events table and publishes to Kafka.
type Relay struct {
	pool         *pgxpool.Pool
	publisher    *eventbus.KafkaPublisher
	log          zerolog.Logger
	batchSize    int
	pollInterval time.Duration
}

// NewRelay creates a new outbox relay.
func NewRelay(pool *pgxpool.Pool, publisher *eventbus.KafkaPublisher, log zerolog.Logger) *Relay {
	initTopicMetrics()
	return &Relay{
		pool:         pool,
		publisher:    publisher,
		log:          log,
		batchSize:    100,
		pollInterval: 1 * time.Second,
	}
}

func initTopicMetrics() {
	for _, topic := range eventbus.AllTopics {
		outboxPublished.WithLabelValues(topic)
		outboxFailures.WithLabelValues(topic)
		outboxPublishDuration.WithLabelValues(topic)
	}
}

// Run starts the relay loop. It blocks until ctx is canceled.
func (r *Relay) Run(ctx context.Context) error {
	r.log.Info().Msg("outbox relay started")

	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info().Msg("outbox relay stopping")
			return nil
		case <-ticker.C:
			if err := r.poll(ctx); err != nil {
				r.log.Error().Err(err).Msg("outbox poll error")
			}
		}
	}
}

func (r *Relay) poll(ctx context.Context) error {
	// Update backlog metric
	var backlog int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE published_at IS NULL`,
	).Scan(&backlog); err != nil {
		return fmt.Errorf("count backlog: %w", err)
	}
	outboxBacklog.Set(float64(backlog))

	if backlog == 0 {
		return nil
	}

	// Fetch unpublished events (no transaction needed — relay is single-instance)
	rows, err := r.pool.Query(ctx,
		`SELECT id, event_type, aggregate_type, aggregate_id, payload_json,
		        schema_version, trace_id, created_at, retry_count
		 FROM outbox_events
		 WHERE published_at IS NULL
		 ORDER BY id ASC
		 LIMIT $1`,
		r.batchSize)
	if err != nil {
		return fmt.Errorf("query outbox: %w", err)
	}
	defer rows.Close()

	var events []outboxRow
	for rows.Next() {
		var row outboxRow
		if err := rows.Scan(&row.ID, &row.EventType, &row.AggregateType,
			&row.AggregateID, &row.PayloadJSON, &row.SchemaVersion,
			&row.TraceID, &row.CreatedAt, &row.RetryCount); err != nil {
			return fmt.Errorf("scan outbox row: %w", err)
		}
		events = append(events, row)
	}

	for _, row := range events {
		if err := r.publishOne(ctx, row); err != nil {
			r.log.Error().Err(err).Int64("id", row.ID).Msg("failed to publish outbox event")
		}
	}

	return nil
}

func (r *Relay) publishOne(ctx context.Context, row outboxRow) error {
	// Extract topic from event_type (e.g., "generation.task.events.task_created" → "generation.task.events")
	topic := extractTopic(row.EventType)

	start := time.Now()

	// Publish raw payload to Kafka
	err := r.publisher.PublishRaw(ctx, topic, row.AggregateID, row.PayloadJSON)
	elapsed := time.Since(start).Seconds()

	outboxPublishDuration.WithLabelValues(topic).Observe(elapsed)

	if err != nil {
		outboxFailures.WithLabelValues(topic).Inc()

		// Update retry count and error
		errMsg := err.Error()
		if _, dbErr := r.pool.Exec(ctx,
			`UPDATE outbox_events SET retry_count = retry_count + 1, last_error = $2 WHERE id = $1`,
			row.ID, errMsg); dbErr != nil {
			r.log.Error().Err(dbErr).Msg("failed to update retry count")
		}
		return fmt.Errorf("publish to %s: %w", topic, err)
	}

	// Mark as published
	if _, err := r.pool.Exec(ctx,
		`UPDATE outbox_events SET published_at = NOW() WHERE id = $1`,
		row.ID); err != nil {
		return fmt.Errorf("mark published: %w", err)
	}

	outboxPublished.WithLabelValues(topic).Inc()
	r.log.Debug().
		Int64("id", row.ID).
		Str("topic", topic).
		Float64("latency_ms", elapsed*1000).
		Msg("outbox event published")

	return nil
}

// extractTopic extracts the Kafka topic from a fully qualified event type.
// e.g., "generation.task.events.task_created" → "generation.task.events"
func extractTopic(eventType string) string {
	for _, topic := range eventbus.AllTopics {
		if strings.HasPrefix(eventType, topic) {
			return topic
		}
	}
	// Fallback: use everything before the last dot
	idx := strings.LastIndex(eventType, ".")
	if idx > 0 {
		return eventType[:idx]
	}
	return eventType
}

// ── Topic Creator ───────────────────────────────────

// EnsureTopics creates all Kafka topics if they don't exist.
func EnsureTopics(brokers []string, log zerolog.Logger) {
	if len(brokers) == 0 {
		return
	}

	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		log.Warn().Err(err).Msg("could not connect to Kafka for topic creation")
		return
	}
	defer conn.Close()

	// Get controller to create topics on
	controller, err := conn.Controller()
	if err != nil {
		log.Warn().Err(err).Msg("could not get Kafka controller")
		return
	}

	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, fmt.Sprintf("%d", controller.Port)))
	if err != nil {
		log.Warn().Err(err).Msg("could not connect to Kafka controller")
		return
	}
	defer controllerConn.Close()

	topicConfigs := make([]kafka.TopicConfig, len(eventbus.AllTopics))
	for i, topic := range eventbus.AllTopics {
		topicConfigs[i] = kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     3,
			ReplicationFactor: 1,
		}
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		if !strings.Contains(err.Error(), "TOPIC_ALREADY_EXISTS") {
			log.Warn().Err(err).Msg("could not create some topics")
		}
	}

	for _, t := range eventbus.AllTopics {
		log.Info().Str("topic", t).Msg("topic ensured")
	}
}
