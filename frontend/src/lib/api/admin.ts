import apiClient from "@/lib/api-client";
import type { ApiResponse } from "@/lib/types/api";

export interface ModerationRecord {
  id: string;
  user_id: string;
  target_type: string;
  target_id: string;
  provider: string;
  status: string;
  categories: unknown;
  score: number | null;
  reason: string | null;
  reviewed_by: string | null;
  reviewed_at: string | null;
  created_at: string;
}

export interface ReviewInput {
  status: "approved" | "rejected";
  reason?: string;
}

export const adminApi = {
  listModeration: (status = "pending") =>
    apiClient
      .get<ApiResponse<ModerationRecord[]>>("/api/admin/moderation", {
        params: { status },
      })
      .then((r) => r.data.data),

  reviewWork: (workId: string, input: ReviewInput) =>
    apiClient
      .post<ApiResponse<ModerationRecord>>(`/api/admin/works/${workId}/review`, input)
      .then((r) => r.data.data),
};
