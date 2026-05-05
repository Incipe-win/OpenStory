"use client";

import { VideoPlayer } from "@/components/works/video-player";
import { StatusBadge } from "@/components/ui/badge";
import { formatDate, formatDuration } from "@/lib/utils/format";
import type { Work } from "@/lib/types/work";
import { Clock, Monitor } from "lucide-react";

interface WorkDetailProps {
  work: Work;
}

export function WorkDetail({ work }: WorkDetailProps) {
  return (
    <div>
      {/* Video player */}
      <div className="aspect-video mb-8">
        {work.file_url ? (
          <VideoPlayer
            src={work.file_url}
            poster={work.thumbnail_url}
            className="w-full h-full"
          />
        ) : (
          <div className="w-full h-full bg-muted flex items-center justify-center cyber-chamfer">
            <p className="text-sm font-mono text-muted-foreground">
              &gt; No video file available
            </p>
          </div>
        )}
      </div>

      {/* Title & Status */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-3xl font-heading font-black uppercase tracking-widest text-foreground mb-2 chromatic-aberration">
            {work.title}
          </h1>
          {work.description && (
            <p className="text-sm font-mono text-muted-foreground max-w-2xl">
              &gt; {work.description}
            </p>
          )}
        </div>
        <StatusBadge status={work.status} />
      </div>

      {/* Metadata */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        <div className="bg-card border border-border cyber-chamfer-sm p-4">
          <p className="text-xs font-mono uppercase tracking-wider text-muted-foreground mb-1">
            Duration
          </p>
          <p className="text-sm font-mono text-accent flex items-center gap-1">
            <Clock className="h-4 w-4" />
            {work.duration_ms ? formatDuration(work.duration_ms) : "N/A"}
          </p>
        </div>
        <div className="bg-card border border-border cyber-chamfer-sm p-4">
          <p className="text-xs font-mono uppercase tracking-wider text-muted-foreground mb-1">
            Resolution
          </p>
          <p className="text-sm font-mono text-accent flex items-center gap-1">
            <Monitor className="h-4 w-4" />
            {work.resolution || "N/A"}
          </p>
        </div>
        <div className="bg-card border border-border cyber-chamfer-sm p-4">
          <p className="text-xs font-mono uppercase tracking-wider text-muted-foreground mb-1">
            Format
          </p>
          <p className="text-sm font-mono text-accent">
            {work.format || "N/A"}
          </p>
        </div>
        <div className="bg-card border border-border cyber-chamfer-sm p-4">
          <p className="text-xs font-mono uppercase tracking-wider text-muted-foreground mb-1">
            Published
          </p>
          <p className="text-sm font-mono text-accent">
            {work.published_at ? formatDate(work.published_at) : "Draft"}
          </p>
        </div>
      </div>
    </div>
  );
}
