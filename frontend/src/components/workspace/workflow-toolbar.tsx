"use client";

import { CyberButton } from "@/components/ui/button";
import { useWorkspaceStore } from "@/lib/stores/workspace-store";
import { useUpdateWorkflow, useValidateWorkflow } from "@/lib/hooks/use-workflows";
import {
  Save,
  CheckCircle,
  Camera,
  AlertCircle,
  Check,
  X,
} from "lucide-react";
import { useState } from "react";

interface WorkflowToolbarProps {
  workflowId: string;
  workflowName: string;
}

export function WorkflowToolbar({ workflowId, workflowName }: WorkflowToolbarProps) {
  const { nodes, edges, isDirty, setDirty } = useWorkspaceStore();
  const updateWorkflow = useUpdateWorkflow();
  const validateWorkflow = useValidateWorkflow();

  const [validation, setValidation] = useState<{
    valid?: boolean;
    error?: string;
    order?: string[];
  } | null>(null);

  const handleSave = () => {
    updateWorkflow.mutate(
      {
        id: workflowId,
        name: workflowName,
        nodes,
        edges,
      },
      { onSuccess: () => setDirty(false) }
    );
  };

  const handleValidate = () => {
    validateWorkflow.mutate(workflowId, {
      onSuccess: (data) => {
        setValidation({
          valid: data.valid,
          error: data.error,
          order: data.execution_order,
        });
      },
    });
  };

  const handleSnapshot = () => {
    // Snapshot is created via API
    import("@/lib/api/workflows").then(({ workflowsApi }) => {
      workflowsApi.snapshot(workflowId);
    });
  };

  return (
    <div className="flex items-center gap-3 px-4 py-2 bg-card border-b border-border">
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
    </div>
  );
}
