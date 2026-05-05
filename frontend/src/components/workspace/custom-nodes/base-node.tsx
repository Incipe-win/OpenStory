"use client";

import { memo } from "react";
import { Handle, Position, type NodeProps, type Node } from "@xyflow/react";
import { cn } from "@/lib/utils/cn";
import { getNodeDef } from "@/lib/utils/nodes";
import type { NodeType } from "@/lib/types/workflow";

export type BaseNodeData = {
  label: string;
  nodeType: NodeType;
  status?: string;
  config?: Record<string, unknown>;
};

export const BaseNode = memo(function BaseNode({
  data,
  selected,
}: NodeProps) {
  const nodeData = data as unknown as BaseNodeData;
  const def = getNodeDef(nodeData.nodeType);
  const Icon = def.icon;

  return (
    <div
      className={cn(
        "bg-card border-2 cyber-chamfer-sm min-w-[180px] transition-all duration-150",
        selected
          ? "border-accent shadow-[var(--shadow-neon)]"
          : def.borderColor.replace("border-", "border-") + " border-opacity-50"
      )}
      style={{
        borderColor: selected ? undefined : def.color + "80",
      }}
    >
      {/* Header */}
      <div
        className="flex items-center gap-2 px-3 py-2 border-b border-border"
        style={{ borderBottomColor: def.color + "40" }}
      >
        <Icon className="h-4 w-4" style={{ color: def.color }} />
        <span className="text-xs font-mono uppercase tracking-wider text-foreground truncate">
          {nodeData.label || def.label}
        </span>
        {nodeData.status === "running" && (
          <span
            className="ml-auto w-2 h-2 rounded-full animate-[pulse_1s_ease-in-out_infinite]"
            style={{ backgroundColor: def.color }}
          />
        )}
      </div>

      {/* Body */}
      <div className="px-3 py-2 text-xs font-mono text-muted-foreground">
        <span className="uppercase tracking-wider opacity-60">
          {def.label}
        </span>
      </div>

      {/* Handles */}
      <Handle
        type="target"
        position={Position.Top}
        className="!bg-accent !border-2 !border-card !w-3 !h-3"
      />
      <Handle
        type="source"
        position={Position.Bottom}
        className="!bg-accent !border-2 !border-card !w-3 !h-3"
      />
    </div>
  );
});

export function createNodeComponent(nodeType: NodeType) {
  const def = getNodeDef(nodeType);
  return function TypedNode(props: NodeProps) {
    return <BaseNode {...props} />;
  };
}
