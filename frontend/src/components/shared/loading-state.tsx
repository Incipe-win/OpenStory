"use client";

import { cn } from "@/lib/utils/cn";
import { Spinner } from "@/components/ui/spinner";

interface LoadingStateProps {
  className?: string;
  message?: string;
}

export function LoadingState({ className, message = "Loading..." }: LoadingStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center py-20 gap-4",
        className
      )}
    >
      <Spinner size="lg" />
      <p className="text-sm font-mono text-accent animate-[blink_1s_step-end_infinite]">
        &gt; {message}
      </p>
    </div>
  );
}
