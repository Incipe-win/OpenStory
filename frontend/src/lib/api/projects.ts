import apiClient from "@/lib/api-client";
import type { ApiResponse, PaginatedResponse } from "@/lib/types/api";
import type { Project, CreateProjectInput, UpdateProjectInput } from "@/lib/types/project";
import type { Work } from "@/lib/types/work";

export const projectsApi = {
  create: (input: CreateProjectInput) =>
    apiClient.post<ApiResponse<Project>>("/api/projects", input).then((r) => r.data.data),

  list: (page = 1, pageSize = 20) =>
    apiClient
      .get<PaginatedResponse<Project>>("/api/projects", { params: { page, page_size: pageSize } })
      .then((r) => r.data),

  get: (id: string) =>
    apiClient.get<ApiResponse<Project>>(`/api/projects/${id}`).then((r) => r.data.data),

  update: (id: string, input: UpdateProjectInput) =>
    apiClient
      .patch<ApiResponse<Project>>(`/api/projects/${id}`, input)
      .then((r) => r.data.data),

  publishWork: (id: string) =>
    apiClient.post<ApiResponse<Work>>(`/api/works/${id}/publish`).then((r) => r.data.data),
};
