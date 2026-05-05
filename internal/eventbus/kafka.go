package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

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
		Key:   []byte(event.AggregateID.String()),
		Value: value,
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
		Key:   []byte(key),
		Value: value,
	})
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
