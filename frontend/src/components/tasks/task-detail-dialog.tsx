"use client";

import { Dialog } from "@/components/ui/dialog";
import { CyberButton } from "@/components/ui/button";
import { TaskStatusBadge } from "@/components/tasks/task-status-badge";
import { useTaskDetail, useTaskEvents, useCancelTask } from "@/lib/hooks/use-tasks";
import { LoadingState } from "@/components/shared/loading-state";
import { formatDate } from "@/lib/utils/format";
import { XCircle } from "lucide-react";
import type { GenerationTask } from "@/lib/types/task";

interface TaskDetailDialogProps {
  taskId: string | null;
  open: boolean;
  onClose: () => void;
}

function TaskContent({ task, events }: { task: GenerationTask; events?: { id: string; event_type: string; created_at: string }[] }) {
  const cancelTask = useCancelTask();

  return (
    <div className="space-y-4">
      {/* Status & Type */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-mono uppercase tracking-wider text-foreground">
            {task.type}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <TaskStatusBadge status={task.status} />
          {(task.status === "pending" || task.status === "queued" || task.status === "running") && (
            <CyberButton
              variant="secondary"
              size="sm"
              onClick={() => cancelTask.mutate(task.id)}
              loading={cancelTask.isPending}
            >
              <XCircle className="h-3 w-3 mr-1" />
              Cancel
            </CyberButton>
          )}
        </div>
      </div>

      {/* Metadata */}
      <div className="grid grid-cols-2 gap-2 text-xs font-mono bg-muted/30 p-3 cyber-chamfer-xs">
        <span className="text-muted-foreground">Provider:</span>
        <span className="text-foreground">{task.provider}</span>
        <span className="text-muted-foreground">Credits:</span>
        <span className="text-accent">{task.cost_credits}</span>
        <span className="text-muted-foreground">Retries:</span>
        <span className="text-foreground">{task.retry_count}</span>
        {task.started_at && (
          <>
            <span className="text-muted-foreground">Started:</span>
            <span className="text-foreground">{formatDate(task.started_at)}</span>
          </>
        )}
        {task.finished_at && (
          <>
            <span className="text-muted-foreground">Finished:</span>
            <span className="text-foreground">{formatDate(task.finished_at)}</span>
          </>
        )}
      </div>

      {/* Input */}
      <div>
        <p className="text-xs font-mono uppercase tracking-wider text-muted-foreground mb-2">
          // Input
        </p>
        <pre className="bg-input border border-border p-3 cyber-chamfer-xs text-xs font-mono text-foreground overflow-auto max-h-32">
          {JSON.stringify(task.input ?? {}, null, 2)}
        </pre>
      </div>

      {/* Output */}
      {task.output && (
        <div>
          <p className="text-xs font-mono uppercase tracking-wider text-muted-foreground mb-2">
            // Output
          </p>
          <pre className="bg-input border border-border p-3 cyber-chamfer-xs text-xs font-mono text-accent overflow-auto max-h-48">
            {JSON.stringify(task.output ?? {}, null, 2)}
          </pre>
        </div>
      )}

      {/* Error */}
      {task.error_message && (
        <div>
          <p className="text-xs font-mono uppercase tracking-wider text-destructive mb-2">
            // Error
          </p>
          <pre className="bg-destructive/5 border border-destructive/30 p-3 cyber-chamfer-xs text-xs font-mono text-destructive overflow-auto max-h-32">
            {task.error_message}
          </pre>
        </div>
      )}

      {/* Events Timeline */}
      {events && events.length > 0 && (
        <div>
          <p className="text-xs font-mono uppercase tracking-wider text-muted-foreground mb-2">
            // Event Timeline
          </p>
          <div className="space-y-1">
            {events.map((event) => (
              <div
                key={event.id}
                className="flex items-center gap-3 text-xs font-mono py-1"
              >
                <span className="w-2 h-2 rounded-full bg-accent" />
                <span className="text-accent uppercase">{event.event_type}</span>
                <span className="text-muted-foreground">
                  {formatDate(event.created_at)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

export function TaskDetailDialog({ taskId, open, onClose }: TaskDetailDialogProps) {
  const { data: task, isLoading } = useTaskDetail(taskId || "");
  const { data: events } = useTaskEvents(taskId || "");

  if (!taskId) return null;

  return (
    <Dialog open={open} onClose={onClose} title="Task Detail" className="max-w-2xl">
      {isLoading && !task && <LoadingState message="Loading task..." />}
      {task && <TaskContent task={task} events={events} />}
    </Dialog>
  );
}
