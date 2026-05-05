"use client";

import { useQuery } from "@tanstack/react-query";
import { creditsApi } from "@/lib/api/credits";
import { useAuthStore } from "@/lib/stores/auth-store";
import { queryKeys } from "@/lib/hooks/query-keys";

export function useCredits() {
  const { accessToken } = useAuthStore();

  return useQuery({
    queryKey: queryKeys.credits.balance,
    queryFn: creditsApi.getBalance,
    enabled: !!accessToken,
    staleTime: 30_000,
  });
}
