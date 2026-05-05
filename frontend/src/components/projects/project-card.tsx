"use client";

import Link from "next/link";
import { cn } from "@/lib/utils/cn";
import { formatDate } from "@/lib/utils/format";
import { CyberCard } from "@/components/ui/card";
import { StatusBadge } from "@/components/ui/badge";
import type { Project } from "@/lib/types/project";
import { FolderKanban, Clock } from "lucide-react";

interface ProjectCardProps {
  project: Project;
  className?: string;
}

export function ProjectCard({ project, className }: ProjectCardProps) {
  return (
    <Link href={`/projects/${project.id}`}>
      <CyberCard
        hoverEffect
        className={cn("p-5 h-full flex flex-col", className)}
      >
        <div className="flex items-start justify-between mb-3">
          <div className="flex items-center gap-2 text-accent">
            <FolderKanban className="h-4 w-4" />
            <span className="text-xs font-mono uppercase tracking-wider">
              {project.status}
            </span>
          </div>
          <StatusBadge status={project.status} />
        </div>

        <h2 className="text-base font-heading font-semibold uppercase tracking-wide text-foreground mb-2 line-clamp-1">
          {project.name}
        </h2>

        {project.description && (
          <p className="text-xs font-mono text-muted-foreground mb-4 line-clamp-2 leading-relaxed">
            {project.description}
          </p>
        )}

        <div className="mt-auto flex items-center gap-1 text-xs text-muted-foreground">
          <Clock className="h-3 w-3" />
          <span>{formatDate(project.created_at)}</span>
        </div>
      </CyberCard>
    </Link>
  );
}
