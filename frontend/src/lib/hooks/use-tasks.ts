"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { tasksApi } from "@/lib/api/tasks";
import { queryKeys } from "@/lib/hooks/query-keys";
import { isTerminal, type GenerationTask } from "@/lib/types/task";
import { POLL_INTERVAL_MS } from "@/lib/config";
import { PAGE_SIZE_DEFAULT } from "@/lib/config";

export function useProjectTasks(projectId: string, page = 1) {
  return useQuery({
    queryKey: queryKeys.projects.tasks(projectId, page),
    queryFn: () => tasksApi.listByProject(projectId, page, PAGE_SIZE_DEFAULT),
    enabled: !!projectId,
  });
}

export function useTaskDetail(id: string) {
  return useQuery<GenerationTask>({
    queryKey: queryKeys.tasks.detail(id),
    queryFn: () => tasksApi.get(id),
    enabled: !!id,
    refetchInterval: (query) => {
      const task = query.state.data;
      if (!task) return POLL_INTERVAL_MS;
      return isTerminal(task.status) ? false : POLL_INTERVAL_MS;
    },
  });
}

export function useTaskEvents(id: string) {
  return useQuery({
    queryKey: queryKeys.tasks.events(id),
    queryFn: () => tasksApi.getEvents(id),
    enabled: !!id,
  });
}

export function useCancelTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => tasksApi.cancel(id),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.tasks.detail(id) });
    },
  });
}
