import { create } from "zustand";
import type { WorkflowNode, WorkflowEdge } from "@/lib/types/workflow";

interface WorkspaceState {
  selectedNodeId: string | null;
  isDirty: boolean;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
  setSelectedNode: (id: string | null) => void;
  setDirty: (dirty: boolean) => void;
  setNodes: (nodes: WorkflowNode[]) => void;
  setEdges: (edges: WorkflowEdge[]) => void;
  updateNodeConfig: (nodeId: string, config: Record<string, unknown>) => void;
  updateNodeName: (nodeId: string, name: string) => void;
  addNode: (node: WorkflowNode) => void;
  removeNode: (nodeId: string) => void;
  reset: () => void;
}

export const useWorkspaceStore = create<WorkspaceState>()((set) => ({
  selectedNodeId: null,
  isDirty: false,
  nodes: [],
  edges: [],
  setSelectedNode: (selectedNodeId) => set({ selectedNodeId }),
  setDirty: (isDirty) => set({ isDirty }),
  setNodes: (nodes) => set({ nodes, isDirty: true }),
  setEdges: (edges) => set({ edges, isDirty: true }),
  updateNodeConfig: (nodeId, config) =>
    set((state) => ({
      nodes: state.nodes.map((n) =>
        n.id === nodeId ? { ...n, config } : n
      ),
      isDirty: true,
    })),
  updateNodeName: (nodeId, name) =>
    set((state) => ({
      nodes: state.nodes.map((n) =>
        n.id === nodeId ? { ...n, name } : n
      ),
      isDirty: true,
    })),
  addNode: (node) =>
    set((state) => ({
      nodes: [...state.nodes, node],
      isDirty: true,
    })),
  removeNode: (nodeId) =>
    set((state) => ({
      nodes: state.nodes.filter((n) => n.id !== nodeId),
      edges: state.edges.filter(
        (e) => e.source_node_id !== nodeId && e.target_node_id !== nodeId
      ),
      selectedNodeId:
        state.selectedNodeId === nodeId ? null : state.selectedNodeId,
      isDirty: true,
    })),
  reset: () =>
    set({
      selectedNodeId: null,
      isDirty: false,
      nodes: [],
      edges: [],
    }),
}));
