"use client";

import { useQuery } from "@tanstack/react-query";
import { feedApi } from "@/lib/api/feed";
import { queryKeys } from "@/lib/hooks/query-keys";
import { PAGE_SIZE_DEFAULT } from "@/lib/config";

export function useFeed(page = 1) {
  return useQuery({
    queryKey: queryKeys.feed.list(page),
    queryFn: () => feedApi.list(page, PAGE_SIZE_DEFAULT),
  });
}
