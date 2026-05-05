"use client";

import { useState, useEffect } from "react";
import { useWorkspaceStore } from "@/lib/stores/workspace-store";
import { getNodeDef } from "@/lib/utils/nodes";
import { CyberInput } from "@/components/ui/input";
import { CyberButton } from "@/components/ui/button";
import { X, Save } from "lucide-react";

export function PropertyPanel() {
  const { selectedNodeId, nodes, updateNodeName, updateNodeConfig, setSelectedNode } =
    useWorkspaceStore();

  const node = nodes.find((n) => n.id === selectedNodeId);
  const [name, setName] = useState(node?.name || "");
  const [configJson, setConfigJson] = useState("");

  useEffect(() => {
    if (node) {
      setName(node.name);
      setConfigJson(JSON.stringify(node.config, null, 2));
    }
  }, [node?.id]);

  if (!node) {
    return (
      <div className="w-72 bg-card border-l border-border p-6 flex flex-col items-center justify-center text-center">
        <p className="text-xs font-mono text-muted-foreground">
          &gt; Select a node
          <br />
          to edit its properties
        </p>
      </div>
    );
  }

  const def = getNodeDef(node.type);
  const Icon = def.icon;

  const handleSave = () => {
    updateNodeName(node.id, name);
    try {
      const config = JSON.parse(configJson);
      updateNodeConfig(node.id, config);
    } catch {
      // Keep current config if JSON is invalid
    }
  };

  return (
    <div className="w-80 bg-card border-l border-border overflow-y-auto">
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-border">
        <div className="flex items-center gap-2">
          <Icon className="h-4 w-4" style={{ color: def.color }} />
          <span className="text-sm font-mono uppercase tracking-wider text-foreground">
            {def.label}
          </span>
        </div>
        <button
          onClick={() => setSelectedNode(null)}
          className="text-muted-foreground hover:text-destructive transition-colors cursor-pointer"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Form */}
      <div className="p-4 space-y-4">
        <CyberInput
          label="Node Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />

        <div>
          <label className="block mb-2 text-xs font-mono uppercase tracking-[0.2em] text-muted-foreground">
            Config (JSON)
          </label>
          <textarea
            value={configJson}
            onChange={(e) => setConfigJson(e.target.value)}
            rows={12}
            className="w-full bg-input border border-border text-sm font-mono text-foreground placeholder:text-muted-foreground p-3 cyber-chamfer-xs focus:border-accent focus:outline-none focus:shadow-[var(--shadow-neon)] transition-all resize-y"
            spellCheck={false}
          />
        </div>

        <div className="grid grid-cols-2 gap-1 text-xs text-muted-foreground">
          <span>Type:</span>
          <span className="font-mono text-foreground">{node.type}</span>
          <span>Position:</span>
          <span className="font-mono text-foreground">
            ({node.position_x}, {node.position_y})
          </span>
          <span>ID:</span>
          <span className="font-mono text-foreground text-[10px] truncate">
            {node.id.slice(0, 12)}...
          </span>
        </div>

        <CyberButton
          variant="glitch"
          size="sm"
          className="w-full"
          onClick={handleSave}
        >
          <Save className="h-4 w-4 mr-1" />
          Apply Changes
        </CyberButton>
      </div>
    </div>
  );
}
