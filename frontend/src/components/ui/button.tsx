"use client";

import { cn } from "@/lib/utils/cn";
import { Loader2 } from "lucide-react";
import { type ButtonHTMLAttributes, forwardRef } from "react";

export interface CyberButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "default" | "secondary" | "outline" | "ghost" | "glitch";
  size?: "sm" | "default" | "lg";
  loading?: boolean;
}

const CyberButton = forwardRef<HTMLButtonElement, CyberButtonProps>(
  ({ className, variant = "default", size = "default", loading, disabled, children, ...props }, ref) => {
    const base =
      "inline-flex items-center justify-center font-mono uppercase tracking-wider transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:opacity-50 disabled:pointer-events-none cursor-pointer";

    const sizes: Record<string, string> = {
      sm: "h-9 px-3 text-xs cyber-chamfer-xs",
      default: "h-11 px-5 text-sm cyber-chamfer-sm",
      lg: "h-14 px-8 text-base cyber-chamfer",
    };

    const variants: Record<string, string> = {
      default:
        "border-2 border-accent text-accent bg-transparent hover:bg-accent hover:text-background hover:shadow-[var(--shadow-neon)]",
      secondary:
        "border-2 border-accent-secondary text-accent-secondary bg-transparent hover:bg-accent-secondary hover:text-background hover:shadow-[var(--shadow-neon-secondary)]",
      outline:
        "border border-border bg-transparent text-muted-foreground hover:border-accent hover:text-accent hover:shadow-[var(--shadow-neon-sm)]",
      ghost:
        "border-none text-muted-foreground hover:bg-accent/10 hover:text-accent",
      glitch:
        "bg-accent text-background font-bold relative overflow-hidden hover:brightness-110 hover:shadow-[var(--shadow-neon-lg)] before:content-[''] before:absolute before:inset-0 before:bg-accent-secondary before:opacity-0 hover:before:opacity-10 before:mix-blend-screen",
    };

    return (
      <button
        ref={ref}
        className={cn(base, sizes[size], variants[variant], className)}
        disabled={disabled || loading}
        {...props}
      >
        {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
        {children}
      </button>
    );
  }
);

CyberButton.displayName = "CyberButton";
export { CyberButton };
