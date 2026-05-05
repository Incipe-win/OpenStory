"use client";

import { StatusBadge } from "@/components/ui/badge";
import type { TaskStatus } from "@/lib/types/task";

interface TaskStatusBadgeProps {
  status: TaskStatus;
}

export function TaskStatusBadge({ status }: TaskStatusBadgeProps) {
  return <StatusBadge status={status} />;
}
