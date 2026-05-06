"use client";

import { type DragEvent } from "react";
import { NODE_PALETTE, getNodeDef } from "@/lib/utils/nodes";

export function NodeTypePalette() {
  const onDragStart = (event: DragEvent, nodeType: string) => {
    event.dataTransfer.setData("application/reactflow-type", nodeType);
    event.dataTransfer.effectAllowed = "move";
  };

  return (
    <div className="w-52 bg-card border-r border-border p-3 overflow-y-auto">
      <p className="text-xs font-mono uppercase tracking-wider text-muted-foreground mb-3 px-1">
        {"// Node Types"}
      </p>
      <div className="space-y-1">
        {NODE_PALETTE.map((type) => {
          const def = getNodeDef(type);
          const Icon = def.icon;
          return (
            <div
              key={type}
              draggable
              onDragStart={(e) => onDragStart(e, type)}
              className="flex items-center gap-2 px-3 py-2 cursor-grab active:cursor-grabbing hover:bg-muted/50 transition-colors cyber-chamfer-xs border border-transparent hover:border-border"
            >
              <Icon className="h-4 w-4" style={{ color: def.color }} />
              <span className="text-xs font-mono text-foreground">
                {def.label}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
