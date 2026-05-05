"use client";

import { useState, use } from "react";
import Link from "next/link";
import { useProject } from "@/lib/hooks/use-projects";
import { useProjectTasks } from "@/lib/hooks/use-tasks";
import { useProjectAssets } from "@/lib/hooks/use-assets";
import { PageContainer } from "@/components/layout/page-container";
import { Topbar } from "@/components/layout/topbar";
import { ProjectTabs } from "@/components/projects/project-tabs";
import { WorkflowCard } from "@/components/workflow/workflow-card";
import { CreateWorkflowDialog } from "@/components/workflow/create-workflow-dialog";
import { CyberButton } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/badge";
import { TaskDetailDialog } from "@/components/tasks/task-detail-dialog";
import { AssetCard } from "@/components/assets/asset-card";
import { LoadingState } from "@/components/shared/loading-state";
import { ErrorState } from "@/components/shared/error-state";
import { EmptyState } from "@/components/shared/empty-state";
import { Pagination } from "@/components/shared/pagination";
import {
  Plus,
  ArrowLeft,
  Clock,
  Workflow,
  ListTodo,
  FolderOpen,
  Play,
} from "lucide-react";
import { formatDate } from "@/lib/utils/format";

export default function ProjectDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const { data: project, isLoading, isError, refetch } = useProject(id);
  const [activeTab, setActiveTab] = useState("workflows");
  const [workflowDialogOpen, setWorkflowDialogOpen] = useState(false);
  const [taskPage, setTaskPage] = useState(1);
  const [assetPage, setAssetPage] = useState(1);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);

  const { data: tasksData } = useProjectTasks(id, taskPage);
  const { data: assetsData } = useProjectAssets(id, assetPage);

  if (isLoading) return <LoadingState message="Loading project..." />;
  if (isError || !project)
    return <ErrorState onRetry={() => refetch()} message="Failed to load project" />;

  return (
    <>
      <Topbar
        title={`Project / ${project.name}`}
        actions={
          <div className="flex items-center gap-2">
            <StatusBadge status={project.status} />
          </div>
        }
      />
      <PageContainer>
        {/* Back link */}
        <Link
          href="/projects"
          className="inline-flex items-center gap-1 text-xs font-mono text-muted-foreground hover:text-accent mb-6 transition-colors"
        >
          <ArrowLeft className="h-3 w-3" />
          All Projects
        </Link>

        {/* Project header */}
        <div className="bg-card border border-border cyber-chamfer p-6 mb-6">
          <h1 className="text-xl font-heading font-bold uppercase tracking-wide text-foreground mb-2">
            {project.name}
          </h1>
          {project.description && (
            <p className="text-sm font-mono text-muted-foreground mb-3">
              &gt; {project.description}
            </p>
          )}
          <div className="flex items-center gap-4 text-xs font-mono text-muted-foreground">
            <span className="flex items-center gap-1">
              <Clock className="h-3 w-3" />
              Created: {formatDate(project.created_at)}
            </span>
          </div>
        </div>

        {/* Tabs */}
        <ProjectTabs
          activeTab={activeTab}
          onTabChange={setActiveTab}
          workflowCount={project.status === "draft" ? undefined : undefined}
          taskCount={tasksData?.meta.total}
          assetCount={assetsData?.meta.total}
        />

        <div className="mt-6">
          {/* Workflows Tab */}
          {activeTab === "workflows" && (
            <div>
              <div className="flex items-center justify-between mb-6">
                <p className="text-xs font-mono text-muted-foreground">
                  &gt; Workflow DAGs for this project
                </p>
                <CyberButton
                  variant="glitch"
                  size="sm"
                  onClick={() => setWorkflowDialogOpen(true)}
                >
                  <Plus className="h-4 w-4 mr-1" />
                  New Workflow
                </CyberButton>
              </div>

              <EmptyState
                title="No workflows yet"
                description="Create a workflow to define your content generation pipeline"
                actionLabel="Create Workflow"
                onAction={() => setWorkflowDialogOpen(true)}
                icon={<Workflow className="h-12 w-12" />}
              />
            </div>
          )}

          {/* Tasks Tab */}
          {activeTab === "tasks" && (
            <div>
              {tasksData && tasksData.data.length > 0 ? (
                <>
                  <div className="space-y-2">
                    {tasksData.data.map((task) => (
                      <div
                        key={task.id}
                        className="flex items-center justify-between bg-card border border-border cyber-chamfer-sm p-4 hover:border-accent/30 transition-colors cursor-pointer"
                        onClick={() => setSelectedTaskId(task.id)}
                      >
                        <div className="flex items-center gap-3">
                          <Play className="h-4 w-4 text-accent" />
                          <div>
                            <p className="text-sm font-mono text-foreground">
                              {task.type}
                            </p>
                            <p className="text-xs text-muted-foreground">
                              {task.provider} / {formatDate(task.created_at)}
                            </p>
                          </div>
                        </div>
                        <div className="flex items-center gap-3">
                          <span className="text-xs text-muted-foreground">
                            {task.cost_credits} credits
                          </span>
                          <StatusBadge status={task.status} />
                        </div>
                      </div>
                    ))}
                  </div>
                  <Pagination
                    page={tasksData.meta.page}
                    pageSize={tasksData.meta.page_size}
                    total={tasksData.meta.total}
                    onPageChange={setTaskPage}
                    className="mt-4"
                  />
                </>
              ) : (
                <EmptyState
                  title="No tasks yet"
                  description="Run a workflow to generate tasks"
                  icon={<ListTodo className="h-12 w-12" />}
                />
              )}
            </div>
          )}

          {/* Assets Tab */}
          {activeTab === "assets" && (
            <div>
              {assetsData && assetsData.data.length > 0 ? (
                <>
                  <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4">
                    {assetsData.data.map((asset) => (
                      <AssetCard key={asset.id} asset={asset} />
                    ))}
                  </div>
                  <Pagination
                    page={assetsData.meta.page}
                    pageSize={assetsData.meta.page_size}
                    total={assetsData.meta.total}
                    onPageChange={setAssetPage}
                    className="mt-4"
                  />
                </>
              ) : (
                <EmptyState
                  title="No assets yet"
                  description="Upload media assets or generate them via workflows"
                  icon={<FolderOpen className="h-12 w-12" />}
                />
              )}
            </div>
          )}
        </div>

        <CreateWorkflowDialog
          open={workflowDialogOpen}
          onClose={() => setWorkflowDialogOpen(false)}
          projectId={id}
        />

        <TaskDetailDialog
          taskId={selectedTaskId}
          open={!!selectedTaskId}
          onClose={() => setSelectedTaskId(null)}
        />
      </PageContainer>
    </>
  );
}
