import apiClient from "@/lib/api-client";
import type { ApiResponse, PaginatedResponse } from "@/lib/types/api";
import type {
  Workflow,
  WorkflowDetail,
  WorkflowNode,
  WorkflowEdge,
  CreateWorkflowInput,
  UpdateWorkflowInput,
  ValidateResult,
  WorkflowRun,
  WorkflowVersion,
} from "@/lib/types/workflow";

type FlatWorkflowDetail = Workflow & {
  nodes?: WorkflowNode[];
  edges?: WorkflowEdge[];
};

function normalizeWorkflowDetail(detail: WorkflowDetail | FlatWorkflowDetail): WorkflowDetail {
  if ("workflow" in detail) return detail;

  const { nodes = [], edges = [], ...workflow } = detail;
  return { workflow, nodes, edges };
}

export const workflowsApi = {
  listByProject: (projectId: string, page = 1, pageSize = 20) =>
    apiClient
      .get<PaginatedResponse<Workflow>>(`/api/projects/${projectId}/workflows`, {
        params: { page, page_size: pageSize },
      })
      .then((r) => r.data),

  create: (projectId: string, input: CreateWorkflowInput) =>
    apiClient
      .post<ApiResponse<Workflow>>(`/api/projects/${projectId}/workflows`, input)
      .then((r) => r.data.data),

  get: (id: string) =>
    apiClient
      .get<ApiResponse<WorkflowDetail | FlatWorkflowDetail>>(`/api/workflows/${id}`)
      .then((r) => normalizeWorkflowDetail(r.data.data)),

  update: (id: string, input: UpdateWorkflowInput) =>
    apiClient
      .put<ApiResponse<WorkflowDetail | FlatWorkflowDetail>>(`/api/workflows/${id}`, input)
      .then((r) => normalizeWorkflowDetail(r.data.data)),

  validate: (id: string) =>
    apiClient
      .post<ApiResponse<ValidateResult>>(`/api/workflows/${id}/validate`)
      .then((r) => r.data.data),

  publish: (id: string) =>
    apiClient
      .post<ApiResponse<WorkflowDetail | FlatWorkflowDetail>>(`/api/workflows/${id}/publish`)
      .then((r) => normalizeWorkflowDetail(r.data.data)),

  run: (id: string) =>
    apiClient
      .post<ApiResponse<WorkflowRun>>(`/api/workflows/${id}/run`, {})
      .then((r) => r.data.data),

  snapshot: (id: string) =>
    apiClient
      .post<ApiResponse<WorkflowVersion>>(`/api/workflows/${id}/snapshot`)
      .then((r) => r.data.data),
};
