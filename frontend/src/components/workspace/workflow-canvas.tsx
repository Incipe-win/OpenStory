"use client";

import { useCallback, useEffect, useRef } from "react";
import {
  ReactFlow,
  ReactFlowProvider,
  Background,
  Controls,
  MiniMap,
  Panel,
  useReactFlow,
  applyEdgeChanges,
  applyNodeChanges,
  useEdgesState,
  useNodesState,
  type Connection,
  type Node,
  type Edge,
  type NodeChange,
  type EdgeChange,
  type OnNodesChange,
  type OnEdgesChange,
  type NodeTypes,
  BackgroundVariant,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { BaseNode } from "@/components/workspace/custom-nodes/base-node";
import { useWorkspaceStore } from "@/lib/stores/workspace-store";
import { getNodeDef } from "@/lib/utils/nodes";
import type { NodeType, WorkflowNode, WorkflowEdge } from "@/lib/types/workflow";
import { v4 as uuidv4 } from "uuid";
import { Maximize2 } from "lucide-react";

const nodeTypes: NodeTypes = {
  idea: BaseNode,
  script: BaseNode,
  character: BaseNode,
  scene: BaseNode,
  storyboard: BaseNode,
  image_prompt: BaseNode,
  image_generation: BaseNode,
  video_generation: BaseNode,
  audio: BaseNode,
  subtitle: BaseNode,
  compose: BaseNode,
};

function toFlowNode(n: WorkflowNode): Node {
  const def = getNodeDef(n.type);
  return {
    id: n.id,
    type: n.type,
    position: { x: n.position_x, y: n.position_y },
    data: {
      label: n.name || def.label,
      nodeType: n.type,
      config: n.config,
    },
  };
}

function mergeFlowNode(existing: Node | undefined, node: WorkflowNode): Node {
  const next = toFlowNode(node);
  if (!existing) return next;

  return {
    ...existing,
    ...next,
    position: next.position,
    data: next.data,
  };
}

function toFlowEdge(e: WorkflowEdge): Edge {
  return {
    id: e.id,
    source: e.source_node_id,
    target: e.target_node_id,
    sourceHandle: e.source_handle,
    targetHandle: e.target_handle,
  };
}

function mergeFlowEdge(existing: Edge | undefined, edge: WorkflowEdge): Edge {
  const next = toFlowEdge(edge);
  return existing ? { ...existing, ...next } : next;
}

function fromFlowNode(node: Node, existing: WorkflowNode | undefined): WorkflowNode {
  const data = node.data as { label?: string; nodeType?: NodeType; config?: Record<string, unknown> };
  return {
    id: node.id,
    type: (data?.nodeType || "idea") as NodeType,
    name: data?.label || getNodeDef(data?.nodeType || "idea").label,
    config: data?.config || existing?.config || getNodeDef(data?.nodeType || "idea").defaultConfig,
    position_x: Math.round(node.position.x),
    position_y: Math.round(node.position.y),
  };
}

function fromFlowEdge(edge: Edge): WorkflowEdge {
  return {
    id: edge.id,
    source_node_id: edge.source,
    target_node_id: edge.target,
    source_handle: edge.sourceHandle || "",
    target_handle: edge.targetHandle || "",
  };
}

function shouldSyncNodeChange(change: NodeChange) {
  return change.type === "position" || change.type === "remove" || change.type === "add" || change.type === "replace";
}

function shouldSyncEdgeChange(change: EdgeChange) {
  return change.type !== "select";
}

export function WorkflowCanvas() {
  return (
    <ReactFlowProvider>
      <WorkflowCanvasSurface />
    </ReactFlowProvider>
  );
}

function WorkflowCanvasSurface() {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const { screenToFlowPosition, fitView } = useReactFlow();
  const previousNodeCount = useRef(0);
  const { nodes: storeNodes, edges: storeEdges, setNodes, setEdges, setSelectedNode, addNode } = useWorkspaceStore();
  const [flowNodes, setFlowNodes] = useNodesState<Node>([]);
  const [flowEdges, setFlowEdges] = useEdgesState<Edge>([]);

  useEffect(() => {
    setFlowNodes((currentNodes) => {
      const currentById = new Map(currentNodes.map((node) => [node.id, node]));
      return storeNodes.map((node) => mergeFlowNode(currentById.get(node.id), node));
    });
  }, [setFlowNodes, storeNodes]);

  useEffect(() => {
    setFlowEdges((currentEdges) => {
      const currentById = new Map(currentEdges.map((edge) => [edge.id, edge]));
      return storeEdges.map((edge) => mergeFlowEdge(currentById.get(edge.id), edge));
    });
  }, [setFlowEdges, storeEdges]);

  const onConnect = useCallback(
    (connection: Connection) => {
      const edge: WorkflowEdge = {
        id: uuidv4(),
        source_node_id: connection.source || "",
        target_node_id: connection.target || "",
        source_handle: connection.sourceHandle || "",
        target_handle: connection.targetHandle || "",
      };
      setEdges([...storeEdges, edge]);
    },
    [setEdges, storeEdges]
  );

  useEffect(() => {
    if (storeNodes.length > previousNodeCount.current && storeNodes.length > 0) {
      window.requestAnimationFrame(() => {
        fitView({ padding: 0.24, duration: 220 });
      });
    }
    previousNodeCount.current = storeNodes.length;
  }, [fitView, storeNodes.length]);

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      setSelectedNode(node.id);
    },
    [setSelectedNode]
  );

  const onPaneClick = useCallback(() => {
    setSelectedNode(null);
  }, [setSelectedNode]);

  const onDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();
      const typeStr = event.dataTransfer.getData("application/reactflow-type");
      if (!typeStr) return;

      const type = typeStr as NodeType;
      const def = getNodeDef(type);
      const position = screenToFlowPosition({
        x: event.clientX,
        y: event.clientY,
      });
      const newNode: WorkflowNode = {
        id: uuidv4(),
        type,
        name: def.label,
        config: { ...def.defaultConfig },
        position_x: Math.round(position.x),
        position_y: Math.round(position.y),
      };
      addNode(newNode);
      setSelectedNode(newNode.id);
    },
    [addNode, screenToFlowPosition, setSelectedNode]
  );

  const onDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
  }, []);

  // Sync flow changes back to store
  const handleNodesChange: OnNodesChange = useCallback(
    (changes) => {
      const updatedNodes = applyNodeChanges(changes, flowNodes);
      setFlowNodes(updatedNodes);

      if (!changes.some(shouldSyncNodeChange)) return;

      const existingMap = new Map(storeNodes.map((n) => [n.id, n]));
      setNodes(updatedNodes.map((n) => fromFlowNode(n, existingMap.get(n.id))));

      if (changes.some((change) => change.type === "remove")) {
        const nodeIds = new Set(updatedNodes.map((node) => node.id));
        setEdges(
          storeEdges.filter(
            (edge) => nodeIds.has(edge.source_node_id) && nodeIds.has(edge.target_node_id)
          )
        );
      }
    },
    [flowNodes, setEdges, setFlowNodes, setNodes, storeEdges, storeNodes]
  );

  const handleEdgesChange: OnEdgesChange = useCallback(
    (changes) => {
      const updatedEdges = applyEdgeChanges(changes, flowEdges);
      setFlowEdges(updatedEdges);

      if (!changes.some(shouldSyncEdgeChange)) return;
      setEdges(updatedEdges.map(fromFlowEdge));
    },
    [flowEdges, setEdges, setFlowEdges]
  );

  return (
    <div
      ref={wrapperRef}
      className="w-full h-full bg-[#101826]"
    >
      <ReactFlow
        className="workspace-flow"
        nodes={flowNodes}
        edges={flowEdges}
        onNodesChange={handleNodesChange}
        onEdgesChange={handleEdgesChange}
        onConnect={onConnect}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        onDrop={onDrop}
        onDragOver={onDragOver}
        nodeTypes={nodeTypes}
        fitView
        deleteKeyCode={["Backspace", "Delete"]}
        multiSelectionKeyCode="Shift"
        snapToGrid
        snapGrid={[10, 10]}
        minZoom={0.2}
        maxZoom={1.8}
        fitViewOptions={{ padding: 0.18, includeHiddenNodes: false }}
      >
        <Background
          variant={BackgroundVariant.Lines}
          gap={28}
          size={1.2}
          color="rgba(0, 212, 255, 0.22)"
        />
        <Controls className="!bg-card !border-border !rounded-none !shadow-[var(--shadow-neon-sm)]" />
        <Panel position="top-right" className="m-3">
          <button
            type="button"
            onClick={() => fitView({ padding: 0.18, duration: 250 })}
            className="h-8 w-8 grid place-items-center bg-card border border-border text-muted-foreground hover:text-accent hover:border-accent/50 transition-colors cyber-chamfer-xs"
            aria-label="Fit view"
          >
            <Maximize2 className="h-4 w-4" />
          </button>
        </Panel>
        <MiniMap
          className="!bg-card !border-border !shadow-[var(--shadow-neon-sm)]"
          maskColor="rgba(10, 12, 18, 0.58)"
          nodeColor={(node) => {
            const data = node.data as { nodeType?: NodeType } | undefined;
            return data?.nodeType ? getNodeDef(data.nodeType).color : "#00ff88";
          }}
        />
      </ReactFlow>
    </div>
  );
}
