"use client";

import { cn } from "@/lib/utils/cn";
import { Search } from "lucide-react";
import { type InputHTMLAttributes, forwardRef } from "react";

const SearchInput = forwardRef<
  HTMLInputElement,
  InputHTMLAttributes<HTMLInputElement>
>(({ className, ...props }, ref) => {
  return (
    <div className="relative">
      <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
      <input
        ref={ref}
        type="text"
        className={cn(
          "w-full h-10 bg-input border border-border pl-10 pr-4 text-sm font-mono text-foreground placeholder:text-muted-foreground cyber-chamfer-xs focus:border-accent focus:outline-none focus:shadow-[var(--shadow-neon-sm)] transition-all",
          className
        )}
        {...props}
      />
    </div>
  );
});

SearchInput.displayName = "SearchInput";
export { SearchInput };
