package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/observability"
)

type DLQPublisher struct {
	writer *kafka.Writer
}

func NewDLQPublisher(brokers []string) *DLQPublisher {
	return &DLQPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        eventbus.TopicDLQEvents,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
		},
	}
}

func (p *DLQPublisher) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

func (p *DLQPublisher) Publish(ctx context.Context, consumerName string, meta MessageMeta, event *eventbus.Event, attempts int, cause error) error {
	if p == nil {
		return nil
	}

	payload := map[string]any{
		"consumer_name": consumerName,
		"source_topic":  meta.Topic,
		"partition":     meta.Partition,
		"offset":        meta.Offset,
		"key":           string(meta.Key),
		"value":         string(meta.Value),
		"attempts":      attempts,
		"error":         cause.Error(),
		"failed_at":     time.Now().UTC(),
	}
	if event != nil {
		payload["event_id"] = event.EventID
		payload["event_type"] = event.EventType
		payload["aggregate_type"] = event.AggregateType
		payload["aggregate_id"] = event.AggregateID
		payload["request_id"] = event.RequestID
		payload["trace_id"] = event.TraceID
		payload["traceparent"] = event.TraceParent
	}

	value, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal dlq payload: %w", err)
	}

	key := fmt.Sprintf("%s:%s:%d:%d", consumerName, meta.Topic, meta.Partition, meta.Offset)
	headers := []kafka.Header{}
	if requestID := observability.RequestIDFromContext(ctx); requestID != "" {
		headers = append(headers, kafka.Header{Key: "x-request-id", Value: []byte(requestID)})
	}
	if traceID := observability.TraceIDFromContext(ctx); traceID != "" {
		headers = append(headers, kafka.Header{Key: "x-trace-id", Value: []byte(traceID)})
	}
	if traceParent := observability.TraceParentFromContext(ctx); traceParent != "" {
		headers = append(headers, kafka.Header{Key: "traceparent", Value: []byte(traceParent)})
	}
	if err := p.writer.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: value, Headers: headers}); err != nil {
		return fmt.Errorf("publish dlq message: %w", err)
	}
	return nil
}
