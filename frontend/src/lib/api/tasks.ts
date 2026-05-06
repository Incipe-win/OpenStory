import apiClient from "@/lib/api-client";
import type { ApiResponse, PaginatedResponse } from "@/lib/types/api";
import type { GenerationTask, TaskEvent, CreateTaskInput } from "@/lib/types/task";

type TaskEventResponse = TaskEvent & { type?: string };

export const tasksApi = {
  create: (input: CreateTaskInput) =>
    apiClient
      .post<ApiResponse<GenerationTask>>("/api/generation/tasks", input)
      .then((r) => r.data.data),

  get: (id: string) =>
    apiClient
      .get<ApiResponse<GenerationTask>>(`/api/generation/tasks/${id}`)
      .then((r) => r.data.data),

  cancel: (id: string) =>
    apiClient
      .post<ApiResponse<GenerationTask>>(`/api/generation/tasks/${id}/cancel`)
      .then((r) => r.data.data),

  getEvents: (id: string) =>
    apiClient
      .get<ApiResponse<TaskEventResponse[]>>(`/api/generation/tasks/${id}/events`)
      .then((r) =>
        r.data.data.map((event) => ({
          ...event,
          event_type: event.event_type || event.type || "",
        }))
      ),

  listByProject: (projectId: string, page = 1, pageSize = 20) =>
    apiClient
      .get<PaginatedResponse<GenerationTask>>(`/api/projects/${projectId}/tasks`, {
        params: { page, page_size: pageSize },
      })
      .then((r) => r.data),
};
