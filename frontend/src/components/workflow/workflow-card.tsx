"use client";

import { CyberCard } from "@/components/ui/card";
import { useToast } from "@/components/ui/toast";
import { StatusBadge } from "@/components/ui/badge";
import { formatDate } from "@/lib/utils/format";
import { useRunWorkflow } from "@/lib/hooks/use-workflows";
import { apiErrorMessage } from "@/lib/api-errors";
import type { Workflow } from "@/lib/types/workflow";
import { Workflow as WorkflowIcon, Clock, Loader2, Play } from "lucide-react";
import { useRouter } from "next/navigation";
import type { KeyboardEvent, MouseEvent } from "react";

interface WorkflowCardProps {
  workflow: Workflow;
}

export function WorkflowCard({ workflow }: WorkflowCardProps) {
  const router = useRouter();
  const toast = useToast();
  const runWorkflow = useRunWorkflow();
  const isRunning = runWorkflow.isPending && runWorkflow.variables === workflow.id;

  const openWorkflow = () => {
    router.push(`/workspace/${workflow.id}`);
  };

  const handleCardKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) return;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openWorkflow();
    }
  };

  const handleRun = (event: MouseEvent<HTMLButtonElement>) => {
    event.preventDefault();
    event.stopPropagation();

    if (workflow.status !== "published") {
      toast.info(
        workflow.status === "validated"
          ? "Please publish the workflow first"
          : "Please validate and publish the workflow first"
      );
      return;
    }

    runWorkflow.mutate(workflow.id, {
      onSuccess: () => {
        toast.success("Workflow run queued", "Opening Tasks to show progress.");
        router.push("/tasks");
      },
      onError: (error) => {
        toast.error("Run failed", apiErrorMessage(error, "Unable to run workflow."));
      },
    });
  };

  return (
      <CyberCard
        hoverEffect
        role="link"
        tabIndex={0}
        onClick={openWorkflow}
        onKeyDown={handleCardKeyDown}
        className="p-5 outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-background"
      >
        <div className="flex items-start justify-between mb-3">
          <div className="flex items-center gap-2 text-accent-tertiary">
            <WorkflowIcon className="h-4 w-4" />
            <span className="text-xs font-mono uppercase tracking-wider">
              v{workflow.current_version}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <StatusBadge status={workflow.status} />
            <button
              type="button"
              aria-label={`Run ${workflow.name}`}
              title="Run workflow"
              onClick={handleRun}
              disabled={isRunning}
              className="inline-flex h-8 w-8 items-center justify-center border border-accent/50 text-accent cyber-chamfer-xs transition-colors hover:bg-accent hover:text-background disabled:pointer-events-none disabled:opacity-50"
            >
              {isRunning ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Play className="h-3.5 w-3.5" />
              )}
            </button>
          </div>
        </div>
        <h3 className="text-base font-heading font-semibold uppercase tracking-wide text-foreground mb-2">
          {workflow.name}
        </h3>
        {workflow.description && (
          <p className="text-xs font-mono text-muted-foreground line-clamp-2 mb-3">
            {workflow.description}
          </p>
        )}
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <Clock className="h-3 w-3" />
          <span>{formatDate(workflow.created_at)}</span>
        </div>
      </CyberCard>
  );
}
