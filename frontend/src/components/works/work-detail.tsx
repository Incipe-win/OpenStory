"use client";

import { useQuery } from "@tanstack/react-query";
import { VideoPlayer } from "@/components/works/video-player";
import { StatusBadge } from "@/components/ui/badge";
import { formatDate, formatDuration } from "@/lib/utils/format";
import type { Work } from "@/lib/types/work";
import { Clock, Monitor, FileText, Image as ImageIcon, Loader2, Music } from "lucide-react";

interface WorkDetailProps {
  work: Work;
}

export function WorkDetail({ work }: WorkDetailProps) {
  const mimeType = (work.format || "").toLowerCase();
  const assetType = metadataString(work.metadata, "asset_type").toLowerCase();
  const isImage = assetType === "image" || mimeType.startsWith("image/");
  const isAudio = assetType === "audio" || mimeType.startsWith("audio/");
  const isText =
    assetType === "text" ||
    mimeType.startsWith("text/") ||
    mimeType.includes("json") ||
    mimeType.includes("xml");
  const textQuery = useQuery({
    queryKey: ["work-text-preview", work.id, work.file_url],
    enabled: isText && !!work.file_url,
    queryFn: async () => {
      const response = await fetch(work.file_url);
      if (!response.ok) {
        throw new Error(`Failed to fetch text work: ${response.status}`);
      }
      const content = await response.text();
      if (mimeType.includes("json")) {
        try {
          return JSON.stringify(JSON.parse(content), null, 2);
        } catch {
          return content;
        }
      }
      return content;
    },
    staleTime: 60_000,
  });

  return (
    <div>
      {/* Primary media */}
      <div className="aspect-video mb-8">
        {isText ? (
          <div className="h-full w-full bg-muted p-4 cyber-chamfer">
            {textQuery.isLoading ? (
              <div className="flex h-full items-center justify-center text-muted-foreground">
                <Loader2 className="h-5 w-5 animate-spin" />
              </div>
            ) : textQuery.isError ? (
              <div className="flex h-full flex-col items-center justify-center gap-2 text-muted-foreground">
                <FileText className="h-12 w-12" />
                <p className="text-xs font-mono">Unable to load text content.</p>
              </div>
            ) : (
              <textarea
                readOnly
                value={textQuery.data || ""}
                className="h-full w-full resize-none bg-background/80 border border-border p-4 text-xs font-mono leading-relaxed text-foreground outline-none cyber-chamfer-sm"
              />
            )}
          </div>
        ) : isImage && work.file_url ? (
          <div className="h-full w-full bg-muted flex items-center justify-center overflow-hidden cyber-chamfer">
            <img src={work.file_url} alt={work.title} className="h-full w-full object-contain" />
          </div>
        ) : isAudio && work.file_url ? (
          <div className="h-full w-full bg-muted flex flex-col items-center justify-center gap-4 cyber-chamfer">
            <Music className="h-16 w-16 text-accent" />
            <audio src={work.file_url} controls className="w-80 max-w-[calc(100%-32px)]" />
          </div>
        ) : work.file_url ? (
          <VideoPlayer
            src={work.file_url}
            poster={work.thumbnail_url}
            className="w-full h-full"
          />
        ) : (
          <div className="w-full h-full bg-muted flex items-center justify-center cyber-chamfer">
            <div className="flex flex-col items-center gap-2 text-muted-foreground">
              <ImageIcon className="h-12 w-12" />
              <p className="text-sm font-mono">&gt; No media file available</p>
            </div>
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

function metadataString(metadata: unknown, key: string) {
  if (!metadata || typeof metadata !== "object" || Array.isArray(metadata)) {
    return "";
  }
  const value = (metadata as Record<string, unknown>)[key];
  return typeof value === "string" ? value : "";
}
