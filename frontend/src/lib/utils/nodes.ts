import type { NodeType } from "@/lib/types/workflow";
import {
  Lightbulb,
  FileText,
  User,
  Map,
  Layout,
  Sparkles,
  Image,
  Video,
  Music,
  Type,
  Combine,
  type LucideIcon,
} from "lucide-react";

export interface NodeDefinition {
  type: NodeType;
  label: string;
  icon: LucideIcon;
  color: string;
  borderColor: string;
  defaultConfig: Record<string, unknown>;
}

export const NODE_DEFINITIONS: Record<NodeType, NodeDefinition> = {
  idea: {
    type: "idea",
    label: "Idea",
    icon: Lightbulb,
    color: "#00ff88",
    borderColor: "border-accent",
    defaultConfig: { prompt: "" },
  },
  script: {
    type: "script",
    label: "Script",
    icon: FileText,
    color: "#00d4ff",
    borderColor: "border-accent-tertiary",
    defaultConfig: { prompt: "", language: "zh" },
  },
  character: {
    type: "character",
    label: "Character",
    icon: User,
    color: "#ff00ff",
    borderColor: "border-accent-secondary",
    defaultConfig: { name: "", traits: "", description: "" },
  },
  scene: {
    type: "scene",
    label: "Scene",
    icon: Map,
    color: "#00d4ff",
    borderColor: "border-accent-tertiary",
    defaultConfig: { setting: "", description: "", characters: [] },
  },
  storyboard: {
    type: "storyboard",
    label: "Storyboard",
    icon: Layout,
    color: "#ff00ff",
    borderColor: "border-accent-secondary",
    defaultConfig: { shots: [] },
  },
  image_prompt: {
    type: "image_prompt",
    label: "Image Prompt",
    icon: Sparkles,
    color: "#00ff88",
    borderColor: "border-accent",
    defaultConfig: { prompt: "", negative_prompt: "", style: "" },
  },
  image_generation: {
    type: "image_generation",
    label: "Image Gen",
    icon: Image,
    color: "#00ff88",
    borderColor: "border-accent",
    defaultConfig: { width: 1024, height: 1024, steps: 30, cfg_scale: 7 },
  },
  video_generation: {
    type: "video_generation",
    label: "Video Gen",
    icon: Video,
    color: "#ff00ff",
    borderColor: "border-accent-secondary",
    defaultConfig: { duration_ms: 5000, fps: 30, width: 1280, height: 720 },
  },
  audio: {
    type: "audio",
    label: "Audio",
    icon: Music,
    color: "#00d4ff",
    borderColor: "border-accent-tertiary",
    defaultConfig: { voice: "", language: "zh", speed: 1.0 },
  },
  subtitle: {
    type: "subtitle",
    label: "Subtitle",
    icon: Type,
    color: "#00d4ff",
    borderColor: "border-accent-tertiary",
    defaultConfig: { format: "srt", language: "zh" },
  },
  compose: {
    type: "compose",
    label: "Compose",
    icon: Combine,
    color: "#ff3366",
    borderColor: "border-destructive",
    defaultConfig: { width: 1920, height: 1080, fps: 30, title: "" },
  },
};

export function getNodeDef(type: NodeType): NodeDefinition {
  return NODE_DEFINITIONS[type];
}

export const NODE_PALETTE: NodeType[] = [
  "idea",
  "script",
  "character",
  "scene",
  "storyboard",
  "image_prompt",
  "image_generation",
  "video_generation",
  "audio",
  "subtitle",
  "compose",
];
