import apiClient from "@/lib/api-client";
import type { ApiResponse } from "@/lib/types/api";
import type { Account } from "@/lib/types/billing";

export const creditsApi = {
  getBalance: () =>
    apiClient.get<ApiResponse<Account>>("/api/credits/balance").then((r) => r.data.data),
};
