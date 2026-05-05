package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Incipe-win/OpenStory/internal/observability"
	"github.com/segmentio/kafka-go"
)

// KafkaPublisher publishes events directly to Kafka topics.
// Used by the outbox-relay, NOT by business code directly.
type KafkaPublisher struct {
	writers map[string]*kafka.Writer
	brokers []string
}

// KafkaEventBus is the Kafka-backed EventBus used by the outbox relay.
type KafkaEventBus = KafkaPublisher

// NewKafkaPublisher creates a publisher with writers for all known topics.
func NewKafkaPublisher(brokers []string) *KafkaPublisher {
	p := &KafkaPublisher{
		writers: make(map[string]*kafka.Writer),
		brokers: brokers,
	}
	for _, topic := range AllTopics {
		p.writers[topic] = &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
			Async:        false,
		}
	}
	return p
}

// NewKafkaEventBus creates the Kafka-backed EventBus implementation.
func NewKafkaEventBus(brokers []string) *KafkaEventBus {
	return NewKafkaPublisher(brokers)
}

// Publish sends a message to the specified Kafka topic.
func (p *KafkaPublisher) Publish(ctx context.Context, topic string, event Event) error {
	w, ok := p.writers[topic]
	if !ok {
		// Fallback: create writer for unknown topic
		w = &kafka.Writer{
			Addr:         kafka.TCP(p.brokers...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
		}
		p.writers[topic] = w
	}

	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := kafka.Message{
		Key:     []byte(event.AggregateID.String()),
		Value:   value,
		Headers: propagationHeaders(ctx, event.RequestID, event.TraceID, event.TraceParent),
	}

	if err := w.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("write kafka message to %s: %w", topic, err)
	}
	return nil
}

// PublishRaw publishes a raw JSON payload to a topic with a given key.
func (p *KafkaPublisher) PublishRaw(ctx context.Context, topic, key string, value []byte) error {
	w, ok := p.writers[topic]
	if !ok {
		w = &kafka.Writer{
			Addr:         kafka.TCP(p.brokers...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
		}
		p.writers[topic] = w
	}

	return w.WriteMessages(ctx, kafka.Message{
		Key:     []byte(key),
		Value:   value,
		Headers: propagationHeaders(ctx, "", "", ""),
	})
}

func propagationHeaders(ctx context.Context, requestID, traceID, traceParent string) []kafka.Header {
	if requestID == "" {
		requestID = observability.RequestIDFromContext(ctx)
	}
	if traceID == "" {
		traceID = observability.TraceIDFromContext(ctx)
	}
	if traceParent == "" {
		traceParent = observability.TraceParentFromContext(ctx)
	}
	headers := make([]kafka.Header, 0, 2)
	if requestID != "" {
		headers = append(headers, kafka.Header{Key: "x-request-id", Value: []byte(requestID)})
	}
	if traceID != "" {
		headers = append(headers, kafka.Header{Key: "x-trace-id", Value: []byte(traceID)})
	}
	if traceParent != "" {
		headers = append(headers, kafka.Header{Key: "traceparent", Value: []byte(traceParent)})
	}
	return headers
}

// Close shuts down all Kafka writers.
func (p *KafkaPublisher) Close() error {
	var lastErr error
	for _, w := range p.writers {
		if err := w.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
