import apiClient from "@/lib/api-client";
import type { ApiResponse } from "@/lib/types/api";
import type {
  Workflow,
  WorkflowDetail,
  CreateWorkflowInput,
  UpdateWorkflowInput,
  ValidateResult,
  WorkflowVersion,
} from "@/lib/types/workflow";

export const workflowsApi = {
  create: (projectId: string, input: CreateWorkflowInput) =>
    apiClient
      .post<ApiResponse<Workflow>>(`/api/projects/${projectId}/workflows`, input)
      .then((r) => r.data.data),

  get: (id: string) =>
    apiClient
      .get<ApiResponse<WorkflowDetail>>(`/api/workflows/${id}`)
      .then((r) => r.data.data),

  update: (id: string, input: UpdateWorkflowInput) =>
    apiClient
      .put<ApiResponse<WorkflowDetail>>(`/api/workflows/${id}`, input)
      .then((r) => r.data.data),

  validate: (id: string) =>
    apiClient
      .post<ApiResponse<ValidateResult>>(`/api/workflows/${id}/validate`)
      .then((r) => r.data.data),

  snapshot: (id: string) =>
    apiClient
      .post<ApiResponse<WorkflowVersion>>(`/api/workflows/${id}/snapshot`)
      .then((r) => r.data.data),
};
