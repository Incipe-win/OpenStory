"use client";

import Link from "next/link";
import { CyberCard } from "@/components/ui/card";
import { formatDate, formatDuration } from "@/lib/utils/format";
import type { Work } from "@/lib/types/work";
import { Play, Clock, Eye } from "lucide-react";

interface FeedCardProps {
  work: Work;
}

export function FeedCard({ work }: FeedCardProps) {
  return (
    <Link href={`/works/${work.id}`}>
      <CyberCard hoverEffect className="overflow-hidden">
        {/* Thumbnail */}
        <div className="aspect-video bg-muted relative overflow-hidden">
          {work.thumbnail_url ? (
            <img
              src={work.thumbnail_url}
              alt={work.title}
              className="w-full h-full object-cover"
              loading="lazy"
            />
          ) : (
            <div className="w-full h-full flex items-center justify-center">
              <Play className="h-12 w-12 text-muted-foreground" />
            </div>
          )}
          <div className="absolute inset-0 bg-black/30 flex items-center justify-center opacity-0 hover:opacity-100 transition-opacity">
            <div className="w-12 h-12 bg-accent/80 cyber-chamfer-xs flex items-center justify-center">
              <Play className="h-6 w-6 text-background ml-0.5" />
            </div>
          </div>
          {work.duration_ms && (
            <span className="absolute bottom-2 right-2 px-2 py-0.5 text-[10px] font-mono bg-card/90 border border-border text-muted-foreground cyber-chamfer-xs">
              {formatDuration(work.duration_ms)}
            </span>
          )}
        </div>

        {/* Info */}
        <div className="p-4">
          <h3 className="text-base font-heading font-semibold uppercase tracking-wide text-foreground mb-2 line-clamp-1">
            {work.title}
          </h3>
          {work.description && (
            <p className="text-xs font-mono text-muted-foreground line-clamp-2 mb-3 leading-relaxed">
              {work.description}
            </p>
          )}
          <div className="flex items-center gap-3 text-[10px] text-muted-foreground">
            <span className="flex items-center gap-1">
              <Clock className="h-3 w-3" />
              {formatDate(work.published_at || work.created_at)}
            </span>
            {work.resolution && (
              <span>{work.resolution}</span>
            )}
          </div>
        </div>
      </CyberCard>
    </Link>
  );
}
