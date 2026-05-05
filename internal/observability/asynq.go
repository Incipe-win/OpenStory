package observability

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

func StartQueueMetrics(ctx context.Context, inspector *asynq.Inspector, queues []string, interval time.Duration, log zerolog.Logger) {
	if inspector == nil || len(queues) == 0 {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, queue := range queues {
					info, err := inspector.GetQueueInfo(queue)
					if err != nil {
						log.Warn().Err(err).Str("queue", queue).Msg("queue metrics scrape failed")
						continue
					}
					SetQueueInfo(queue, info.Pending, info.Active, info.Scheduled, info.Retry, info.Archived, info.Latency)
				}
			}
		}
	}()
}
