"use client";

import Link from "next/link";
import { CyberCard } from "@/components/ui/card";
import { StatusBadge } from "@/components/ui/badge";
import { formatDate } from "@/lib/utils/format";
import type { Workflow } from "@/lib/types/workflow";
import { Workflow as WorkflowIcon, Clock } from "lucide-react";

interface WorkflowCardProps {
  workflow: Workflow;
}

export function WorkflowCard({ workflow }: WorkflowCardProps) {
  return (
    <Link href={`/workspace/${workflow.id}`}>
      <CyberCard hoverEffect className="p-5">
        <div className="flex items-start justify-between mb-3">
          <div className="flex items-center gap-2 text-accent-tertiary">
            <WorkflowIcon className="h-4 w-4" />
            <span className="text-xs font-mono uppercase tracking-wider">
              v{workflow.current_version}
            </span>
          </div>
          <StatusBadge status={workflow.status} />
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
    </Link>
  );
}
