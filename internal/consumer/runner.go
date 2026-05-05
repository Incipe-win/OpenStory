package consumer

import (
	"context"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/observability"
)

type Runner struct {
	def    Definition
	reader *kafka.Reader
	store  Store
	dlq    *DLQPublisher
	proc   *Processor
	log    zerolog.Logger
}

func NewRunner(def Definition, brokers []string, store Store, dlq *DLQPublisher, log zerolog.Logger) *Runner {
	initConsumerMetrics(def)
	return &Runner{
		def: def,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			Topic:          def.Topic,
			GroupID:        def.GroupID,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: 0, // explicit commit after successful processing/DLQ handoff
		}),
		store: store,
		dlq:   dlq,
		proc:  NewProcessor(def, store),
		log:   log.With().Str("consumer", def.Name).Str("topic", def.Topic).Logger(),
	}
}

func (r *Runner) Close() error {
	if r.reader == nil {
		return nil
	}
	return r.reader.Close()
}

func (r *Runner) Run(ctx context.Context) error {
	r.log.Info().Str("group", r.def.GroupID).Msg("consumer started")
	for {
		msg, err := r.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			r.log.Error().Err(err).Msg("fetch message failed")
			continue
		}

		if err := r.processMessage(ctx, msg); err != nil {
			r.log.Error().
				Err(err).
				Int("partition", msg.Partition).
				Int64("offset", msg.Offset).
				Msg("message processing failed")
		}
	}
}

func (r *Runner) processMessage(ctx context.Context, msg kafka.Message) error {
	started := time.Now()
	meta := MessageMeta{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       msg.Key,
		Value:     msg.Value,
	}

	if msg.HighWaterMark > 0 {
		lag := msg.HighWaterMark - msg.Offset - 1
		if lag < 0 {
			lag = 0
		}
		consumerLag.WithLabelValues(r.def.Name, r.def.Topic, strconv.Itoa(msg.Partition)).Set(float64(lag))
	}

	event, processed, err := r.proc.Process(ctx, meta)
	if event != nil {
		ctx = observability.ContextWithIDs(ctx, event.RequestID, event.TraceID)
	}
	if err != nil {
		return r.handleFailure(ctx, meta, event, err, started)
	}

	if !processed {
		observeMessage(r.def, "duplicate", started)
		r.log.Debug().
			Str("request_id", observability.RequestIDFromContext(ctx)).
			Str("trace_id", observability.TraceIDFromContext(ctx)).
			Int("partition", msg.Partition).
			Int64("offset", msg.Offset).
			Msg("duplicate message skipped")
		return r.reader.CommitMessages(ctx, msg)
	}

	observeMessage(r.def, "processed", started)
	r.log.Debug().
		Str("request_id", observability.RequestIDFromContext(ctx)).
		Str("trace_id", observability.TraceIDFromContext(ctx)).
		Int("partition", msg.Partition).
		Int64("offset", msg.Offset).
		Msg("message processed")
	return r.reader.CommitMessages(ctx, msg)
}

func (r *Runner) handleFailure(ctx context.Context, meta MessageMeta, event *eventbus.Event, cause error, started time.Time) error {
	observeFailure(r.def)
	observeMessage(r.def, "failed", started)
	r.log.Warn().
		Err(cause).
		Str("request_id", observability.RequestIDFromContext(ctx)).
		Str("trace_id", observability.TraceIDFromContext(ctx)).
		Int("partition", meta.Partition).
		Int64("offset", meta.Offset).
		Msg("message routed to dlq")

	attempts, err := r.store.RecordFailure(ctx, r.def.Name, meta, event, cause)
	if err != nil {
		return err
	}
	if err := r.dlq.Publish(ctx, r.def.Name, meta, event, attempts, cause); err != nil {
		return err
	}

	// The failed source message is committed only after the DLQ handoff succeeds.
	return r.reader.CommitMessages(ctx, kafka.Message{
		Topic:     meta.Topic,
		Partition: meta.Partition,
		Offset:    meta.Offset,
	})
}
