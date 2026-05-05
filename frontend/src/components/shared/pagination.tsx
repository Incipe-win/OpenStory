"use client";

import { cn } from "@/lib/utils/cn";
import { ChevronLeft, ChevronRight } from "lucide-react";

interface PaginationProps {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
  className?: string;
}

export function Pagination({
  page,
  pageSize,
  total,
  onPageChange,
  className,
}: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  if (totalPages <= 1) return null;

  return (
    <div className={cn("flex items-center justify-center gap-2", className)}>
      <button
        onClick={() => onPageChange(page - 1)}
        disabled={page <= 1}
        className="p-2 text-muted-foreground hover:text-accent disabled:opacity-30 disabled:pointer-events-none transition-colors cursor-pointer"
      >
        <ChevronLeft className="h-4 w-4" />
      </button>

      <span className="text-sm font-mono text-muted-foreground">
        {page} / {totalPages}
      </span>

      <button
        onClick={() => onPageChange(page + 1)}
        disabled={page >= totalPages}
        className="p-2 text-muted-foreground hover:text-accent disabled:opacity-30 disabled:pointer-events-none transition-colors cursor-pointer"
      >
        <ChevronRight className="h-4 w-4" />
      </button>
    </div>
  );
}
