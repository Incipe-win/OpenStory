"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { workflowsApi } from "@/lib/api/workflows";
import { queryKeys } from "@/lib/hooks/query-keys";
import type { CreateWorkflowInput, UpdateWorkflowInput } from "@/lib/types/workflow";
import { PAGE_SIZE_DEFAULT } from "@/lib/config";

export function useWorkflow(id: string) {
  return useQuery({
    queryKey: queryKeys.workflows.detail(id),
    queryFn: () => workflowsApi.get(id),
    enabled: !!id,
  });
}

export function useProjectWorkflows(projectId: string, page = 1) {
  return useQuery({
    queryKey: queryKeys.workflows.listByProject(projectId, page, PAGE_SIZE_DEFAULT),
    queryFn: () => workflowsApi.listByProject(projectId, page, PAGE_SIZE_DEFAULT),
    enabled: !!projectId,
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
      queryClient.invalidateQueries({
        queryKey: queryKeys.workflows.byProject(projectId),
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
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => workflowsApi.validate(id),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.workflows.detail(id) });
      queryClient.invalidateQueries({ queryKey: ["workflows"] });
    },
  });
}

export function useRunWorkflow() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => workflowsApi.run(id),
    onSuccess: (run, id) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.workflows.detail(id) });
      queryClient.invalidateQueries({ queryKey: ["projects"] });
      if (run.task?.project_id) {
        queryClient.invalidateQueries({
          queryKey: ["projects", run.task.project_id, "tasks"],
        });
      }
    },
  });
}
