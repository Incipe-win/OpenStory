"use client";

import { useState } from "react";
import { PageContainer } from "@/components/layout/page-container";
import { Topbar } from "@/components/layout/topbar";
import { TaskStatusBadge } from "@/components/tasks/task-status-badge";
import { TaskDetailDialog } from "@/components/tasks/task-detail-dialog";
import { LoadingState } from "@/components/shared/loading-state";
import { EmptyState } from "@/components/shared/empty-state";
import { useProjects } from "@/lib/hooks/use-projects";
import { useProjectTasks } from "@/lib/hooks/use-tasks";
import { formatDate } from "@/lib/utils/format";
import { ListTodo, Play } from "lucide-react";

export default function TasksPage() {
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const { data: projectsData } = useProjects(1);

  // Collect tasks from the first page of each project
  const allTasks: Array<{ task: ReturnType<typeof useProjectTasks>["data"]; projectId: string }> = [];
  const projectList = projectsData?.data || [];

  return (
    <>
      <Topbar title="Tasks" />
      <PageContainer>
        <p className="text-xs font-mono text-muted-foreground mb-6">
          &gt; All generation tasks across projects
        </p>

        {projectList.length === 0 ? (
          <EmptyState
            title="No tasks"
            description="Create a project first, then run workflows to generate tasks"
            icon={<ListTodo className="h-12 w-12" />}
          />
        ) : (
          <div className="space-y-6">
            {projectList.map((project) => (
              <TaskProjectSection
                key={project.id}
                projectId={project.id}
                projectName={project.name}
                onTaskClick={setSelectedTaskId}
              />
            ))}
          </div>
        )}

        <TaskDetailDialog
          taskId={selectedTaskId}
          open={!!selectedTaskId}
          onClose={() => setSelectedTaskId(null)}
        />
      </PageContainer>
    </>
  );
}

function TaskProjectSection({
  projectId,
  projectName,
  onTaskClick,
}: {
  projectId: string;
  projectName: string;
  onTaskClick: (id: string) => void;
}) {
  const { data, isLoading } = useProjectTasks(projectId, 1);

  if (isLoading) return <LoadingState message={`Loading ${projectName}...`} />;
  if (!data || data.data.length === 0) return null;

  return (
    <div>
      <h3 className="text-sm font-heading font-semibold uppercase tracking-wider text-foreground mb-3">
        {projectName}
      </h3>
      <div className="space-y-2">
        {data.data.map((task) => (
          <div
            key={task.id}
            className="flex items-center justify-between bg-card border border-border cyber-chamfer-sm p-4 hover:border-accent/30 transition-colors cursor-pointer"
            onClick={() => onTaskClick(task.id)}
          >
            <div className="flex items-center gap-3">
              <Play className="h-4 w-4 text-accent" />
              <div>
                <p className="text-sm font-mono text-foreground">{task.type}</p>
                <p className="text-xs text-muted-foreground">
                  {task.provider} / {formatDate(task.created_at)}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <span className="text-xs text-muted-foreground">{task.cost_credits} credits</span>
              <TaskStatusBadge status={task.status} />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
