"use client";

import { cn } from "@/lib/utils/cn";
import { type InputHTMLAttributes, forwardRef } from "react";

export interface CyberInputProps
  extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  prefix?: string;
  error?: string;
}

const CyberInput = forwardRef<HTMLInputElement, CyberInputProps>(
  ({ className, label, prefix = ">", error, id, ...props }, ref) => {
    return (
      <div className="w-full">
        {label && (
          <label
            htmlFor={id}
            className="block mb-2 text-xs font-mono uppercase tracking-[0.2em] text-muted-foreground"
          >
            {label}
          </label>
        )}
        <div className="relative">
          {prefix && (
            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-accent font-mono text-sm select-none">
              {prefix}
            </span>
          )}
          <input
            ref={ref}
            id={id}
            className={cn(
              "w-full h-11 bg-input border font-mono text-sm text-foreground placeholder:text-muted-foreground cyber-chamfer-sm transition-all duration-200",
              prefix ? "pl-8 pr-4" : "px-4",
              error
                ? "border-destructive shadow-[var(--shadow-neon-destructive)] focus:border-destructive"
                : "border-border focus:border-accent focus:shadow-[var(--shadow-neon)]",
              "focus:outline-none",
              className
            )}
            {...props}
          />
        </div>
        {error && (
          <p className="mt-1.5 text-xs text-destructive font-mono">
            // {error}
          </p>
        )}
      </div>
    );
  }
);

CyberInput.displayName = "CyberInput";
export { CyberInput };
