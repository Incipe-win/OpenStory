"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { projectsApi } from "@/lib/api/projects";
import { queryKeys } from "@/lib/hooks/query-keys";
import type { CreateProjectInput, UpdateProjectInput } from "@/lib/types/project";
import { PAGE_SIZE_DEFAULT } from "@/lib/config";

export function useProjects(page = 1) {
  return useQuery({
    queryKey: queryKeys.projects.list(page, PAGE_SIZE_DEFAULT),
    queryFn: () => projectsApi.list(page, PAGE_SIZE_DEFAULT),
  });
}

export function useProject(id: string) {
  return useQuery({
    queryKey: queryKeys.projects.detail(id),
    queryFn: () => projectsApi.get(id),
    enabled: !!id,
  });
}

export function useCreateProject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (input: CreateProjectInput) => projectsApi.create(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.projects.all });
    },
  });
}

export function useUpdateProject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, ...input }: { id: string } & UpdateProjectInput) =>
      projectsApi.update(id, input),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.projects.detail(id) });
      queryClient.invalidateQueries({ queryKey: queryKeys.projects.all });
    },
  });
}
