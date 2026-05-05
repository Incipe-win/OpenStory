export type TaskStatus =
  | "pending"
  | "queued"
  | "running"
  | "succeeded"
  | "failed"
  | "canceled";

export type TaskType =
  | "idea"
  | "script"
  | "character"
  | "characters"
  | "scene"
  | "storyboard"
  | "image_prompt"
  | "video_prompt"
  | "creative_pipeline"
  | "image_generation"
  | "video_generation"
  | "audio"
  | "subtitle"
  | "compose";

export interface GenerationTask {
  id: string;
  user_id: string;
  project_id: string;
  workflow_id: string | null;
  node_id: string | null;
  type: TaskType;
  provider: string;
  status: TaskStatus;
  idempotency_key: string;
  input: Record<string, unknown>;
  output: Record<string, unknown> | null;
  error_message: string | null;
  cost_credits: number;
  retry_count: number;
  started_at: string | null;
  finished_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface TaskEvent {
  id: string;
  task_id: string;
  event_type: string;
  payload: unknown;
  created_at: string;
}

export interface CreateTaskInput {
  project_id: string;
  workflow_id?: string;
  node_id?: string;
  type: string;
  provider?: string;
  idempotency_key: string;
  input: Record<string, unknown>;
}

export const TERMINAL_STATUSES: TaskStatus[] = [
  "succeeded",
  "failed",
  "canceled",
];

export function isTerminal(status: TaskStatus): boolean {
  return TERMINAL_STATUSES.includes(status);
}
