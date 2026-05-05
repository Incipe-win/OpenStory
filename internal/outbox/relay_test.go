package outbox

import "testing"

func TestExtractTopic(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		want      string
	}{
		{name: "generation task", eventType: "generation.task.events.task_created", want: "generation.task.events"},
		{name: "asset", eventType: "asset.events.asset_created", want: "asset.events"},
		{name: "fallback", eventType: "custom.topic.event", want: "custom.topic"},
		{name: "no dot", eventType: "event", want: "event"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractTopic(tt.eventType); got != tt.want {
				t.Fatalf("extractTopic(%q) = %q, want %q", tt.eventType, got, tt.want)
			}
		})
	}
}
