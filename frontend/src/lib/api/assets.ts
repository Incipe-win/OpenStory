import apiClient from "@/lib/api-client";
import type { ApiResponse, PaginatedResponse } from "@/lib/types/api";
import type { Asset, UploadUrlResponse, UploadUrlInput, ComposeInput, ComposeResult } from "@/lib/types/asset";

export const assetsApi = {
  getUploadUrl: (input: UploadUrlInput) =>
    apiClient
      .post<ApiResponse<UploadUrlResponse>>("/api/assets/upload-url", input)
      .then((r) => r.data.data),

  get: (id: string) =>
    apiClient.get<ApiResponse<Asset>>(`/api/assets/${id}`).then((r) => r.data.data),

  listByProject: (projectId: string, page = 1, pageSize = 20) =>
    apiClient
      .get<PaginatedResponse<Asset>>(`/api/projects/${projectId}/assets`, {
        params: { page, page_size: pageSize },
      })
      .then((r) => r.data),

  compose: (projectId: string, input: ComposeInput) =>
    apiClient
      .post<ApiResponse<ComposeResult>>(`/api/projects/${projectId}/compose`, input)
      .then((r) => r.data.data),

  getComposeStatus: (taskId: string) =>
    apiClient
      .get<ApiResponse<ComposeResult>>(`/api/compose/${taskId}`)
      .then((r) => r.data.data),
};
