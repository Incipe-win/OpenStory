"use client";

import { cn } from "@/lib/utils/cn";

interface Tab {
  id: string;
  label: string;
  count?: number;
}

interface TabsProps {
  tabs: Tab[];
  activeTab: string;
  onTabChange: (id: string) => void;
  className?: string;
}

export function Tabs({ tabs, activeTab, onTabChange, className }: TabsProps) {
  return (
    <div className={cn("flex border-b border-border", className)}>
      {tabs.map((tab) => (
        <button
          key={tab.id}
          onClick={() => onTabChange(tab.id)}
          className={cn(
            "relative px-4 py-3 text-sm font-mono uppercase tracking-wider transition-colors cursor-pointer",
            activeTab === tab.id
              ? "text-accent"
              : "text-muted-foreground hover:text-foreground"
          )}
        >
          {tab.label}
          {tab.count !== undefined && (
            <span className="ml-2 text-xs text-muted-foreground">
              [{tab.count}]
            </span>
          )}
          {activeTab === tab.id && (
            <span className="absolute bottom-0 left-0 right-0 h-0.5 bg-accent shadow-[var(--shadow-neon)]" />
          )}
        </button>
      ))}
    </div>
  );
}
