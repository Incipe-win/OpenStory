import apiClient from "@/lib/api-client";
import type { ApiResponse } from "@/lib/types/api";
import type { Work } from "@/lib/types/work";

export const worksApi = {
  get: (id: string) =>
    apiClient.get<ApiResponse<Work>>(`/api/works/${id}`).then((r) => r.data.data),

  publish: (id: string) =>
    apiClient.post<ApiResponse<Work>>(`/api/works/${id}/publish`).then((r) => r.data.data),
};
