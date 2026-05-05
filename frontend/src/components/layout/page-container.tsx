"use client";

import { cn } from "@/lib/utils/cn";

interface PageContainerProps {
  children: React.ReactNode;
  className?: string;
}

export function PageContainer({ children, className }: PageContainerProps) {
  return (
    <div
      className={cn(
        "flex-1 overflow-auto p-6 circuit-bg",
        className
      )}
    >
      {children}
    </div>
  );
}
