package consumer

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	consumerLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "openstory",
		Subsystem: "consumer",
		Name:      "kafka_lag",
		Help:      "Approximate Kafka consumer lag from the message high watermark",
	}, []string{"consumer", "topic", "partition"})
	consumerDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "openstory",
		Subsystem: "consumer",
		Name:      "consume_duration_seconds",
		Help:      "Time spent processing one consumed message",
		Buckets:   prometheus.DefBuckets,
	}, []string{"consumer", "topic", "status"})
	consumerFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openstory",
		Subsystem: "consumer",
		Name:      "failures_total",
		Help:      "Total consumer processing failures",
	}, []string{"consumer", "topic"})
	consumerMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openstory",
		Subsystem: "consumer",
		Name:      "messages_total",
		Help:      "Total consumed messages by result status",
	}, []string{"consumer", "topic", "status"})
	consumerFailureRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "openstory",
		Subsystem: "consumer",
		Name:      "failure_rate",
		Help:      "Consumer failure ratio since process start",
	}, []string{"consumer", "topic"})
)

var inMemoryRates sync.Map

type rateState struct {
	total    float64
	failures float64
}

func initConsumerMetrics(def Definition) {
	consumerFailures.WithLabelValues(def.Name, def.Topic)
	consumerMessages.WithLabelValues(def.Name, def.Topic, "processed")
	consumerMessages.WithLabelValues(def.Name, def.Topic, "duplicate")
	consumerMessages.WithLabelValues(def.Name, def.Topic, "failed")
	consumerFailureRate.WithLabelValues(def.Name, def.Topic).Set(0)
}

func observeMessage(def Definition, status string, started time.Time) {
	consumerMessages.WithLabelValues(def.Name, def.Topic, status).Inc()
	consumerDuration.WithLabelValues(def.Name, def.Topic, status).Observe(time.Since(started).Seconds())
	updateFailureRate(def, status == "failed")
}

func observeFailure(def Definition) {
	consumerFailures.WithLabelValues(def.Name, def.Topic).Inc()
}

func updateFailureRate(def Definition, failed bool) {
	key := def.Name + ":" + def.Topic
	value, _ := inMemoryRates.LoadOrStore(key, &rateState{})
	state := value.(*rateState)
	state.total++
	if failed {
		state.failures++
	}
	if state.total == 0 {
		consumerFailureRate.WithLabelValues(def.Name, def.Topic).Set(0)
		return
	}
	consumerFailureRate.WithLabelValues(def.Name, def.Topic).Set(state.failures / state.total)
}
