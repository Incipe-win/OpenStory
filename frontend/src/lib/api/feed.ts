import apiClient from "@/lib/api-client";
import type { PaginatedResponse } from "@/lib/types/api";
import type { Work } from "@/lib/types/work";

export const feedApi = {
  list: (page = 1, pageSize = 20) =>
    apiClient
      .get<PaginatedResponse<Work>>("/api/feed", { params: { page, page_size: pageSize } })
      .then((r) => r.data),
};
