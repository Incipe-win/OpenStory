"use client";

import { cn } from "@/lib/utils/cn";
import type { TaskStatus } from "@/lib/types/task";

interface StatusBadgeProps {
  status: TaskStatus | string;
  className?: string;
}

const statusStyles: Record<string, string> = {
  pending: "bg-muted text-muted-foreground border-muted",
  queued: "bg-accent-tertiary/10 text-accent-tertiary border-accent-tertiary/30",
  validated: "bg-accent/10 text-accent border-accent/40",
  running:
    "bg-accent/10 text-accent border-accent/30 animate-[pulse_2s_ease-in-out_infinite]",
  succeeded: "bg-accent/15 text-accent border-accent/50",
  failed: "bg-destructive/10 text-destructive border-destructive/30",
  canceled: "bg-muted text-muted-foreground border-muted line-through",
};

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const style =
    statusStyles[status] || statusStyles.pending;

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 px-2.5 py-0.5 text-xs font-mono uppercase tracking-wider border cyber-chamfer-xs",
        style,
        className
      )}
    >
      {status === "running" && (
        <span className="w-1.5 h-1.5 rounded-full bg-accent animate-[pulse_2s_ease-in-out_infinite]" />
      )}
      {status}
    </span>
  );
}
