package observability

import (
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	apiRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openstory",
		Subsystem: "api",
		Name:      "requests_total",
		Help:      "Total HTTP API requests",
	}, []string{"method", "route", "status"})
	apiRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "openstory",
		Subsystem: "api",
		Name:      "request_duration_seconds",
		Help:      "HTTP API request latency",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route", "status"})

	taskEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openstory",
		Subsystem: "task",
		Name:      "events_total",
		Help:      "Generation task terminal events by status",
	}, []string{"task_type", "provider", "status"})
	taskDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "openstory",
		Subsystem: "task",
		Name:      "duration_seconds",
		Help:      "Generation task runtime from worker start to terminal status",
		Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
	}, []string{"task_type", "provider", "status"})
	taskSuccessRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "openstory",
		Subsystem: "task",
		Name:      "success_rate",
		Help:      "Task success ratio since process start",
	}, []string{"task_type", "provider"})

	queueBacklog = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "openstory",
		Subsystem: "queue",
		Name:      "backlog_total",
		Help:      "Asynq queue backlog by state",
	}, []string{"queue", "state"})
	queueLatency = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "openstory",
		Subsystem: "queue",
		Name:      "latency_seconds",
		Help:      "Asynq queue latency based on oldest pending task",
	}, []string{"queue"})

	providerCalls = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openstory",
		Subsystem: "provider",
		Name:      "calls_total",
		Help:      "Provider calls by status",
	}, []string{"provider", "capability", "status"})
	providerDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "openstory",
		Subsystem: "provider",
		Name:      "duration_seconds",
		Help:      "Provider call duration",
		Buckets:   prometheus.DefBuckets,
	}, []string{"provider", "capability", "status"})
	providerErrorRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "openstory",
		Subsystem: "provider",
		Name:      "error_rate",
		Help:      "Provider error ratio since process start",
	}, []string{"provider", "capability"})

	ffmpegRuns = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openstory",
		Subsystem: "ffmpeg",
		Name:      "runs_total",
		Help:      "FFmpeg command executions by operation and status",
	}, []string{"operation", "status"})
	ffmpegDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "openstory",
		Subsystem: "ffmpeg",
		Name:      "duration_seconds",
		Help:      "FFmpeg command duration",
		Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
	}, []string{"operation", "status"})

	billingOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "openstory",
		Subsystem: "billing",
		Name:      "operations_total",
		Help:      "Credit billing operations by status",
	}, []string{"operation", "status"})
	billingFailureRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "openstory",
		Subsystem: "billing",
		Name:      "failure_rate",
		Help:      "Credit billing failure ratio since process start",
	}, []string{"operation"})
)

type ratioState struct {
	mu       sync.Mutex
	total    float64
	failures float64
}

var (
	taskRatios     sync.Map
	providerRatios sync.Map
	billingRatios  sync.Map
)

func ObserveAPIRequest(method, route string, status int, duration time.Duration) {
	statusCode := strconv.Itoa(status)
	apiRequests.WithLabelValues(method, route, statusCode).Inc()
	apiRequestDuration.WithLabelValues(method, route, statusCode).Observe(duration.Seconds())
}

func ObserveTask(taskType, provider, status string, duration time.Duration) {
	taskEvents.WithLabelValues(taskType, provider, status).Inc()
	taskDuration.WithLabelValues(taskType, provider, status).Observe(duration.Seconds())
	key := taskType + ":" + provider
	value, _ := taskRatios.LoadOrStore(key, &ratioState{})
	state := value.(*ratioState)
	state.mu.Lock()
	defer state.mu.Unlock()
	state.total++
	if status != "succeeded" {
		state.failures++
	}
	taskSuccessRate.WithLabelValues(taskType, provider).Set(1 - state.failures/state.total)
}

func SetQueueInfo(queue string, pending, active, scheduled, retry, archived int, latency time.Duration) {
	queueBacklog.WithLabelValues(queue, "pending").Set(float64(pending))
	queueBacklog.WithLabelValues(queue, "active").Set(float64(active))
	queueBacklog.WithLabelValues(queue, "scheduled").Set(float64(scheduled))
	queueBacklog.WithLabelValues(queue, "retry").Set(float64(retry))
	queueBacklog.WithLabelValues(queue, "archived").Set(float64(archived))
	queueLatency.WithLabelValues(queue).Set(latency.Seconds())
}

func ObserveProviderCall(provider, capability, status string, duration time.Duration) {
	providerCalls.WithLabelValues(provider, capability, status).Inc()
	providerDuration.WithLabelValues(provider, capability, status).Observe(duration.Seconds())
	key := provider + ":" + capability
	value, _ := providerRatios.LoadOrStore(key, &ratioState{})
	state := value.(*ratioState)
	state.mu.Lock()
	defer state.mu.Unlock()
	state.total++
	if status != "succeeded" {
		state.failures++
	}
	providerErrorRate.WithLabelValues(provider, capability).Set(state.failures / state.total)
}

func ObserveFFmpeg(operation, status string, duration time.Duration) {
	ffmpegRuns.WithLabelValues(operation, status).Inc()
	ffmpegDuration.WithLabelValues(operation, status).Observe(duration.Seconds())
}

func ObserveBilling(operation string, failed bool) {
	status := "succeeded"
	if failed {
		status = "failed"
	}
	billingOperations.WithLabelValues(operation, status).Inc()
	value, _ := billingRatios.LoadOrStore(operation, &ratioState{})
	state := value.(*ratioState)
	state.mu.Lock()
	defer state.mu.Unlock()
	state.total++
	if failed {
		state.failures++
	}
	billingFailureRate.WithLabelValues(operation).Set(state.failures / state.total)
}
