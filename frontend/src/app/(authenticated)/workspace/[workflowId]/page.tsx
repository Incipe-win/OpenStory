"use client";

import { use, useEffect } from "react";
import { useWorkflow } from "@/lib/hooks/use-workflows";
import { useWorkspaceStore } from "@/lib/stores/workspace-store";
import { WorkflowCanvas } from "@/components/workspace/workflow-canvas";
import { PropertyPanel } from "@/components/workspace/property-panel";
import { NodeTypePalette } from "@/components/workspace/node-type-palette";
import { WorkflowToolbar } from "@/components/workspace/workflow-toolbar";
import { LoadingState } from "@/components/shared/loading-state";
import { ErrorState } from "@/components/shared/error-state";
import { ArrowLeft } from "lucide-react";
import Link from "next/link";

export default function WorkspacePage({
  params,
}: {
  params: Promise<{ workflowId: string }>;
}) {
  const { workflowId } = use(params);
  const { data, isLoading, isError, refetch } = useWorkflow(workflowId);
  const { setNodes, setEdges, reset } = useWorkspaceStore();

  useEffect(() => {
    if (data) {
      setNodes(data.nodes);
      setEdges(data.edges);
      // Mark clean after loading
      setTimeout(() => useWorkspaceStore.getState().setDirty(false), 100);
    }
    return () => {
      reset();
    };
  }, [data?.workflow?.id]);

  if (isLoading) return <LoadingState message="Loading workflow..." />;
  if (isError || !data)
    return <ErrorState onRetry={() => refetch()} message="Failed to load workflow" />;

  return (
    <div className="flex flex-col h-screen">
      {/* Toolbar */}
      <div className="flex items-center gap-2">
        <Link
          href={`/projects/${data.workflow.project_id}`}
          className="px-4 text-muted-foreground hover:text-accent transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <WorkflowToolbar
          workflowId={workflowId}
          workflowName={data.workflow.name}
        />
      </div>

      {/* Main workspace */}
      <div className="flex-1 flex overflow-hidden">
        <NodeTypePalette />
        <div className="flex-1">
          <WorkflowCanvas />
        </div>
        <PropertyPanel />
      </div>
    </div>
  );
}
