"use client";

import { type DragEvent } from "react";
import { Plus } from "lucide-react";
import { NODE_PALETTE, getNodeDef } from "@/lib/utils/nodes";
import { useWorkspaceStore } from "@/lib/stores/workspace-store";
import type { NodeType, WorkflowNode } from "@/lib/types/workflow";
import { v4 as uuidv4 } from "uuid";

export function NodeTypePalette() {
  const { addNode, nodes, setSelectedNode } = useWorkspaceStore();

  const createNode = (nodeType: NodeType): WorkflowNode => {
    const def = getNodeDef(nodeType);
    const offset = nodes.length * 32;

    return {
      id: uuidv4(),
      type: nodeType,
      name: def.label,
      config: { ...def.defaultConfig },
      position_x: 80 + (offset % 240),
      position_y: 80 + (offset % 180),
    };
  };

  const addPaletteNode = (nodeType: NodeType) => {
    const node = createNode(nodeType);
    addNode(node);
    setSelectedNode(node.id);
  };

  const onDragStart = (event: DragEvent, nodeType: NodeType) => {
    event.dataTransfer.setData("application/reactflow-type", nodeType);
    event.dataTransfer.setData("text/plain", nodeType);
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
            <button
              key={type}
              type="button"
              draggable
              onDragStart={(e) => onDragStart(e, type)}
              onClick={() => addPaletteNode(type)}
              className="w-full flex items-center gap-2 px-3 py-2 cursor-grab active:cursor-grabbing hover:bg-muted/50 transition-colors cyber-chamfer-xs border border-transparent hover:border-border text-left"
            >
              <Icon className="h-4 w-4" style={{ color: def.color }} />
              <span className="flex-1 text-xs font-mono text-foreground">
                {def.label}
              </span>
              <Plus className="h-3.5 w-3.5 text-muted-foreground" />
            </button>
          );
        })}
      </div>
    </div>
  );
}
