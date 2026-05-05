"use client";

import { cn } from "@/lib/utils/cn";
import { Loader2 } from "lucide-react";

interface SpinnerProps {
  className?: string;
  size?: "sm" | "default" | "lg";
}

export function Spinner({ className, size = "default" }: SpinnerProps) {
  const sizes: Record<string, string> = {
    sm: "h-4 w-4",
    default: "h-8 w-8",
    lg: "h-12 w-12",
  };

  return (
    <Loader2
      className={cn(
        "animate-spin text-accent",
        sizes[size],
        className
      )}
    />
  );
}
