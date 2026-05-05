"use client";

import { useState } from "react";
import { PageContainer } from "@/components/layout/page-container";
import { Topbar } from "@/components/layout/topbar";
import { CyberButton } from "@/components/ui/button";
import { ProjectList } from "@/components/projects/project-list";
import { CreateProjectDialog } from "@/components/projects/create-project-dialog";
import { Pagination } from "@/components/shared/pagination";
import { LoadingState } from "@/components/shared/loading-state";
import { ErrorState } from "@/components/shared/error-state";
import { EmptyState } from "@/components/shared/empty-state";
import { useProjects } from "@/lib/hooks/use-projects";
import { Plus } from "lucide-react";

export default function ProjectsPage() {
  const [page, setPage] = useState(1);
  const [dialogOpen, setDialogOpen] = useState(false);
  const { data, isLoading, isError, refetch } = useProjects(page);

  return (
    <>
      <Topbar title="Projects" />
      <PageContainer>
        <div className="flex items-center justify-between mb-6">
          <p className="text-xs font-mono text-muted-foreground">
            &gt; {data?.meta.total ?? 0} project(s) found
          </p>
          <CyberButton variant="glitch" size="sm" onClick={() => setDialogOpen(true)}>
            <Plus className="h-4 w-4 mr-1" />
            New Project
          </CyberButton>
        </div>

        {isLoading && <LoadingState message="Loading projects..." />}
        {isError && <ErrorState onRetry={() => refetch()} />}
        {data && data.data.length === 0 && (
          <EmptyState
            title="No projects yet"
            description="Create your first project to get started"
            actionLabel="Create Project"
            onAction={() => setDialogOpen(true)}
          />
        )}
        {data && data.data.length > 0 && (
          <>
            <ProjectList projects={data.data} />
            <Pagination
              page={data.meta.page}
              pageSize={data.meta.page_size}
              total={data.meta.total}
              onPageChange={setPage}
              className="mt-6"
            />
          </>
        )}

        <CreateProjectDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
      </PageContainer>
    </>
  );
}
