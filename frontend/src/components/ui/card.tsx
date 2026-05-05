"use client";

import { cn } from "@/lib/utils/cn";
import type { HTMLAttributes } from "react";

export interface CyberCardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "terminal" | "holographic";
  hoverEffect?: boolean;
}

export function CyberCard({
  className,
  variant = "default",
  hoverEffect,
  children,
  ...props
}: CyberCardProps) {
  const base = "transition-all duration-300";

  const variants: Record<string, string> = {
    default:
      "bg-card border border-border cyber-chamfer",
    terminal:
      "bg-background border border-border cyber-chamfer pt-10 relative",
    holographic:
      "bg-muted/30 border border-accent/30 cyber-chamfer backdrop-blur-sm relative",
  };

  const hoverClass = hoverEffect
    ? "hover:-translate-y-0.5 hover:border-accent hover:shadow-[var(--shadow-neon)] cursor-pointer"
    : "";

  return (
    <div className={cn(base, variants[variant], hoverClass, className)} {...props}>
      {variant === "terminal" && (
        <div className="absolute top-0 left-0 right-0 h-8 bg-muted flex items-center gap-1.5 px-3 border-b border-border">
          <div className="w-3 h-3 rounded-full bg-destructive" />
          <div className="w-3 h-3 rounded-full bg-yellow-500" />
          <div className="w-3 h-3 rounded-full bg-accent" />
        </div>
      )}
      {variant === "holographic" && (
        <>
          <div className="absolute top-0 left-0 w-4 h-4 border-t border-l border-accent/50" />
          <div className="absolute top-0 right-0 w-4 h-4 border-t border-r border-accent/50" />
          <div className="absolute bottom-0 left-0 w-4 h-4 border-b border-l border-accent/50" />
          <div className="absolute bottom-0 right-0 w-4 h-4 border-b border-r border-accent/50" />
        </>
      )}
      {children}
    </div>
  );
}
