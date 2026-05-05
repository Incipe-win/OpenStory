"use client";

import { Tabs } from "@/components/ui/tabs";

interface ProjectTabsProps {
  activeTab: string;
  onTabChange: (tab: string) => void;
  workflowCount?: number;
  taskCount?: number;
  assetCount?: number;
}

export function ProjectTabs({
  activeTab,
  onTabChange,
  workflowCount,
  taskCount,
  assetCount,
}: ProjectTabsProps) {
  const tabs = [
    { id: "workflows", label: "Workflows", count: workflowCount },
    { id: "tasks", label: "Tasks", count: taskCount },
    { id: "assets", label: "Assets", count: assetCount },
  ];

  return <Tabs tabs={tabs} activeTab={activeTab} onTabChange={onTabChange} />;
}
