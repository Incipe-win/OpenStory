"use client";

import { useCallback, useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  addEdge,
  type Connection,
  type Node,
  type Edge,
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

export function WorkflowCanvas() {
  const { nodes: storeNodes, edges: storeEdges, setNodes, setEdges, setSelectedNode, addNode } = useWorkspaceStore();

  const initialNodes = useMemo(() => storeNodes.map(toFlowNode), [storeNodes]);
  const initialEdges = useMemo(
    (): Edge[] =>
      storeEdges.map((e) => ({
        id: e.id,
        source: e.source_node_id,
        target: e.target_node_id,
        sourceHandle: e.source_handle,
        targetHandle: e.target_handle,
      })),
    [storeEdges]
  );

  const [flowNodes, setFlowNodes, onNodesChange] = useNodesState(initialNodes);
  const [flowEdges, setFlowEdges, onEdgesChange] = useEdgesState(initialEdges);

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
    [storeEdges, setEdges]
  );

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

      const position = { x: event.clientX - 300, y: event.clientY - 100 };
      const newNode: WorkflowNode = {
        id: uuidv4(),
        type,
        name: def.label,
        config: { ...def.defaultConfig },
        position_x: Math.round(position.x),
        position_y: Math.round(position.y),
      };
      addNode(newNode);
    },
    [addNode]
  );

  const onDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
  }, []);

  // Sync flow changes back to store
  const handleNodesChange: OnNodesChange = useCallback(
    (changes) => {
      onNodesChange(changes);
      // Convert and sync positions to store
      const updatedNodes = flowNodes.map((n) => {
        const change = changes.find((c) => "id" in c && c.id === n.id);
        if (change && change.type === "position" && "position" in change && change.position) {
          return { ...n, position: change.position };
        }
        return n;
      });
      const existingMap = new Map(storeNodes.map((n) => [n.id, n]));
      const newStoreNodes = updatedNodes.map((n) => fromFlowNode(n, existingMap.get(n.id)));
      useWorkspaceStore.getState().setNodes(newStoreNodes);
    },
    [onNodesChange, flowNodes, storeNodes]
  );

  return (
    <div className="w-full h-full">
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        onNodesChange={handleNodesChange}
        onEdgesChange={onEdgesChange}
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
      >
        <Background
          variant={BackgroundVariant.Dots}
          gap={20}
          size={1}
          color="#2a2a3a"
        />
        <Controls className="!bg-card !border-border !rounded-none" />
        <MiniMap
          className="!bg-card !border-border"
          maskColor="rgba(0,0,0,0.7)"
          nodeColor={(node) => {
            const data = node.data as { nodeType?: NodeType } | undefined;
            return data?.nodeType ? getNodeDef(data.nodeType).color : "#00ff88";
          }}
        />
      </ReactFlow>
    </div>
  );
}
