"use client";

import { CyberButton } from "@/components/ui/button";
import { useToast } from "@/components/ui/toast";
import { useWorkspaceStore } from "@/lib/stores/workspace-store";
import { useRunWorkflow, useUpdateWorkflow, useValidateWorkflow } from "@/lib/hooks/use-workflows";
import { apiErrorMessage } from "@/lib/api-errors";
import {
  Save,
  CheckCircle,
  Camera,
  Check,
  X,
  Play,
} from "lucide-react";
import { useState } from "react";
import { useRouter } from "next/navigation";
import type { WorkflowExecutionNode } from "@/lib/types/workflow";

interface WorkflowToolbarProps {
  workflowId: string;
  workflowName: string;
  workflowStatus: string;
}

export function WorkflowToolbar({ workflowId, workflowName, workflowStatus }: WorkflowToolbarProps) {
  const { nodes, edges, isDirty, setDirty } = useWorkspaceStore();
  const updateWorkflow = useUpdateWorkflow();
  const validateWorkflow = useValidateWorkflow();
  const runWorkflow = useRunWorkflow();
  const router = useRouter();
  const toast = useToast();

  const [validation, setValidation] = useState<{
    valid?: boolean;
    error?: string;
    order?: WorkflowExecutionNode[];
  } | null>(() => (workflowStatus === "validated" ? { valid: true } : null));

  const canRun = validation?.valid === true && !isDirty;

  const handleSave = () => {
    updateWorkflow.mutate(
      {
        id: workflowId,
        name: workflowName,
        nodes,
        edges,
      },
      {
        onSuccess: () => {
          setDirty(false);
          setValidation(null);
          toast.success("Workflow saved", "Validate it again before running.");
        },
        onError: (error) => {
          toast.error("Save failed", apiErrorMessage(error, "Unable to save workflow."));
        },
      }
    );
  };

  const handleValidate = () => {
    if (isDirty) {
      toast.info("Save required", "Save the workflow before validating.");
      return;
    }

    validateWorkflow.mutate(workflowId, {
      onSuccess: (data) => {
        setValidation({
          valid: data.valid,
          error: data.error,
          order: data.execution_order,
        });
        if (data.valid) {
          toast.success("Workflow validated", "Run is now available.");
        } else {
          toast.error("Validation failed", data.error || "Please fix the workflow.");
        }
      },
      onError: (error) => {
        toast.error("Validation failed", apiErrorMessage(error, "Unable to validate workflow."));
      },
    });
  };

  const handleSnapshot = () => {
    // Snapshot is created via API
    import("@/lib/api/workflows").then(({ workflowsApi }) => {
      workflowsApi.snapshot(workflowId);
    });
  };

  const handleRun = () => {
    if (!canRun) {
      toast.info(
        "Please validate the workflow first",
        isDirty ? "Save and validate the latest changes before running." : undefined
      );
      return;
    }

    runWorkflow.mutate(workflowId, {
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
    <div className="flex flex-1 items-center gap-3 px-4 py-2 bg-card border-b border-border">
      {/* Workflow name */}
      <div className="flex items-center gap-2 mr-4">
        <span className="text-sm font-heading font-semibold uppercase tracking-wide text-foreground">
          {workflowName}
        </span>
        {isDirty && (
          <span className="text-xs text-accent animate-[blink_1s_step-end_infinite]">
            *
          </span>
        )}
      </div>

      <div className="flex-1" />

      {/* Validation result */}
      {validation && (
        <div className="flex items-center gap-2 text-xs font-mono">
          {validation.valid ? (
            <>
              <Check className="h-4 w-4 text-accent" />
              <span className="text-accent">Valid</span>
              {validation.order && (
                <span className="text-muted-foreground">
                  [{validation.order.length} nodes]
                </span>
              )}
            </>
          ) : (
            <>
              <X className="h-4 w-4 text-destructive" />
              <span className="text-destructive">{validation.error}</span>
            </>
          )}
        </div>
      )}

      {/* Actions */}
      <CyberButton variant="ghost" size="sm" onClick={handleSnapshot}>
        <Camera className="h-4 w-4 mr-1" />
        Snapshot
      </CyberButton>

      <CyberButton variant="outline" size="sm" onClick={handleValidate} loading={validateWorkflow.isPending}>
        <CheckCircle className="h-4 w-4 mr-1" />
        Validate
      </CyberButton>

      <CyberButton
        variant="glitch"
        size="sm"
        onClick={handleSave}
        loading={updateWorkflow.isPending}
        disabled={!isDirty}
      >
        <Save className="h-4 w-4 mr-1" />
        Save
      </CyberButton>

      <CyberButton
        variant="glitch"
        size="sm"
        onClick={handleRun}
        loading={runWorkflow.isPending}
        disabled={!canRun}
        title={!canRun ? "Validate the saved workflow before running" : "Run workflow"}
      >
        <Play className="h-4 w-4 mr-1" />
        Run
      </CyberButton>
    </div>
  );
}
