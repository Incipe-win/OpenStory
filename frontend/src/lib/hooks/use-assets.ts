"use client";

import { useQuery } from "@tanstack/react-query";
import { assetsApi } from "@/lib/api/assets";
import { queryKeys } from "@/lib/hooks/query-keys";
import { PAGE_SIZE_DEFAULT } from "@/lib/config";

export function useProjectAssets(projectId: string, page = 1) {
  return useQuery({
    queryKey: queryKeys.projects.assets(projectId, page),
    queryFn: () => assetsApi.listByProject(projectId, page, PAGE_SIZE_DEFAULT),
    enabled: !!projectId,
  });
}

export function useAssetDetail(id: string) {
  return useQuery({
    queryKey: queryKeys.assets.detail(id),
    queryFn: () => assetsApi.get(id),
    enabled: !!id,
  });
}
