"use client";

import { useCallback, useEffect, useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
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

function toFlowEdge(e: WorkflowEdge): Edge {
  return {
    id: e.id,
    source: e.source_node_id,
    target: e.target_node_id,
    sourceHandle: e.source_handle,
    targetHandle: e.target_handle,
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

function fromFlowEdge(edge: Edge): WorkflowEdge {
  return {
    id: edge.id,
    source_node_id: edge.source,
    target_node_id: edge.target,
    source_handle: edge.sourceHandle || "",
    target_handle: edge.targetHandle || "",
  };
}

export function WorkflowCanvas() {
  const { nodes: storeNodes, edges: storeEdges, setNodes, setEdges, setSelectedNode, addNode } = useWorkspaceStore();

  const initialNodes = useMemo(() => storeNodes.map(toFlowNode), [storeNodes]);
  const initialEdges = useMemo((): Edge[] => storeEdges.map(toFlowEdge), [storeEdges]);

  const [flowNodes, setFlowNodes] = useNodesState(initialNodes);
  const [flowEdges, setFlowEdges] = useEdgesState(initialEdges);

  useEffect(() => {
    setFlowNodes(storeNodes.map(toFlowNode));
  }, [setFlowNodes, storeNodes]);

  useEffect(() => {
    setFlowEdges(storeEdges.map(toFlowEdge));
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
      setFlowEdges((current) => addEdge(toFlowEdge(edge), current));
      setEdges([...storeEdges, edge]);
    },
    [setEdges, setFlowEdges, storeEdges]
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
      setFlowNodes((current) => {
        const updatedNodes = applyNodeChanges(changes, current);
        const existingMap = new Map(storeNodes.map((n) => [n.id, n]));
        setNodes(updatedNodes.map((n) => fromFlowNode(n, existingMap.get(n.id))));
        return updatedNodes;
      });
    },
    [setFlowNodes, setNodes, storeNodes]
  );

  const handleEdgesChange: OnEdgesChange = useCallback(
    (changes) => {
      setFlowEdges((current) => {
        const updatedEdges = applyEdgeChanges(changes, current);
        setEdges(updatedEdges.map(fromFlowEdge));
        return updatedEdges;
      });
    },
    [setEdges, setFlowEdges]
  );

  return (
    <div className="w-full h-full">
      <ReactFlow
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
