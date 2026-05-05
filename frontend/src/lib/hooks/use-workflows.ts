"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { workflowsApi } from "@/lib/api/workflows";
import { queryKeys } from "@/lib/hooks/query-keys";
import type { CreateWorkflowInput, UpdateWorkflowInput } from "@/lib/types/workflow";

export function useWorkflow(id: string) {
  return useQuery({
    queryKey: queryKeys.workflows.detail(id),
    queryFn: () => workflowsApi.get(id),
    enabled: !!id,
  });
}

export function useCreateWorkflow() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      projectId,
      ...input
    }: { projectId: string } & CreateWorkflowInput) =>
      workflowsApi.create(projectId, input),
    onSuccess: (_, { projectId }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projects.detail(projectId),
      });
    },
  });
}

export function useUpdateWorkflow() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, ...input }: { id: string } & UpdateWorkflowInput) =>
      workflowsApi.update(id, input),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.workflows.detail(id) });
    },
  });
}

export function useValidateWorkflow() {
  return useMutation({
    mutationFn: (id: string) => workflowsApi.validate(id),
  });
}
