"use client";

import { cn } from "@/lib/utils/cn";

interface AssetTypeFilterProps {
  activeFilter: string;
  onFilterChange: (filter: string) => void;
}

const filters = [
  { id: "all", label: "All" },
  { id: "image", label: "Images" },
  { id: "video", label: "Videos" },
  { id: "audio", label: "Audio" },
];

export function AssetTypeFilter({ activeFilter, onFilterChange }: AssetTypeFilterProps) {
  return (
    <div className="flex gap-1">
      {filters.map((f) => (
        <button
          key={f.id}
          onClick={() => onFilterChange(f.id)}
          className={cn(
            "px-3 py-1.5 text-xs font-mono uppercase tracking-wider transition-all cyber-chamfer-xs border cursor-pointer",
            activeFilter === f.id
              ? "bg-accent/10 text-accent border-accent/30"
              : "text-muted-foreground border-transparent hover:border-border hover:text-foreground"
          )}
        >
          {f.label}
        </button>
      ))}
    </div>
  );
}
