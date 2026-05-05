"use client";

import { cn } from "@/lib/utils/cn";
import { CyberButton } from "@/components/ui/button";
import { FolderOpen } from "lucide-react";

interface EmptyStateProps {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  actionLabel?: string;
  onAction?: () => void;
  className?: string;
}

export function EmptyState({
  icon,
  title,
  description,
  actionLabel,
  onAction,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center py-20 px-4 text-center",
        className
      )}
    >
      <div className="mb-4 text-muted-foreground">
        {icon || <FolderOpen className="h-12 w-12" />}
      </div>
      <p className="text-sm font-mono text-muted-foreground mb-2">
        &gt; {title}
      </p>
      {description && (
        <p className="text-xs font-mono text-muted-foreground/60 mb-6 max-w-md">
          {description}
        </p>
      )}
      {actionLabel && onAction && (
        <CyberButton variant="outline" size="sm" onClick={onAction}>
          {actionLabel}
        </CyberButton>
      )}
    </div>
  );
}
