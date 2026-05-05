"use client";

import { cn } from "@/lib/utils/cn";
import { CyberButton } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

interface ErrorStateProps {
  message?: string;
  onRetry?: () => void;
  className?: string;
}

export function ErrorState({
  message = "Something went wrong",
  onRetry,
  className,
}: ErrorStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center py-20 px-4 text-center",
        className
      )}
    >
      <AlertTriangle className="h-12 w-12 text-destructive mb-4" />
      <p className="text-sm font-mono text-destructive mb-2 chromatic-aberration">
        &gt; ERROR: {message}
      </p>
      {onRetry && (
        <CyberButton variant="outline" size="sm" onClick={onRetry} className="mt-4">
          Retry
        </CyberButton>
      )}
    </div>
  );
}
