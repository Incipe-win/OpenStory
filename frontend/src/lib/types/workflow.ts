import type { GenerationTask } from "@/lib/types/task";

export type NodeType =
  | "idea"
  | "script"
  | "character"
  | "scene"
  | "storyboard"
  | "image_prompt"
  | "image_generation"
  | "video_generation"
  | "audio"
  | "subtitle"
  | "compose";

export interface WorkflowNode {
  id: string;
  workflow_id?: string;
  type: NodeType;
  name: string;
  config: Record<string, unknown>;
  position_x: number;
  position_y: number;
  created_at?: string;
}

export interface WorkflowEdge {
  id: string;
  workflow_id?: string;
  source_node_id: string;
  target_node_id: string;
  source_handle: string;
  target_handle: string;
}

export interface Workflow {
  id: string;
  project_id: string;
  user_id: string;
  name: string;
  description: string;
  status: string;
  current_version: number;
  created_at: string;
  updated_at: string;
}

export interface WorkflowDetail {
  workflow: Workflow;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
}

export interface CreateWorkflowInput {
  name: string;
  description?: string;
}

export interface UpdateWorkflowInput {
  name: string;
  description?: string;
  status?: string;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
}

export interface WorkflowExecutionNode {
  id: string;
  type: NodeType;
  name: string;
}

export interface ValidateResult {
  valid: boolean;
  error?: string;
  status?: string;
  project_id?: string;
  execution_order?: WorkflowExecutionNode[];
  node_schemas?: Record<string, unknown>;
}

export interface WorkflowRun {
  task_id: string;
  task?: GenerationTask;
}

export interface WorkflowVersion {
  id: string;
  workflow_id: string;
  version: number;
  snapshot: WorkflowDetail;
  created_at: string;
}
