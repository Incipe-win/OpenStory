"use client";

import { cn } from "@/lib/utils/cn";
import { formatBytes, formatDuration, formatDate } from "@/lib/utils/format";
import type { Asset } from "@/lib/types/asset";
import { Image as ImageIcon, Video, Music, FileText, Send } from "lucide-react";
import { StatusBadge } from "@/components/ui/badge";

interface AssetCardProps {
  asset: Asset;
  onClick?: () => void;
  onSubmitToFeed?: (asset: Asset) => void;
  submitLoading?: boolean;
  className?: string;
}

const typeIcons: Record<string, React.ReactNode> = {
  image: <ImageIcon className="h-5 w-5" />,
  video: <Video className="h-5 w-5" />,
  audio: <Music className="h-5 w-5" />,
  text: <FileText className="h-5 w-5" />,
};

export function AssetCard({ asset, onClick, onSubmitToFeed, submitLoading, className }: AssetCardProps) {
  const type = asset.type.toLowerCase();
  const mimeType = asset.mime_type.toLowerCase();
  const isImage = type === "image" || mimeType.startsWith("image/");
  const isVideo = type === "video" || mimeType.startsWith("video/");
  const canSubmit = asset.status !== "pending_review" && asset.status !== "published";
  const submitLabel =
    asset.status === "pending_review"
      ? "In Review"
      : asset.status === "published"
        ? "Published"
        : "Submit to Feed";

  return (
    <div
      onClick={onClick}
      className={cn(
        "bg-card border border-border cyber-chamfer-sm overflow-hidden hover:border-accent/30 transition-all hover:-translate-y-0.5 cursor-pointer group",
        className
      )}
    >
      {/* Thumbnail */}
      <div className="aspect-video bg-muted flex items-center justify-center relative overflow-hidden">
        {isImage && asset.thumbnail_url ? (
          <img
            src={asset.thumbnail_url}
            alt={asset.name}
            className="w-full h-full object-cover"
            loading="lazy"
          />
        ) : isVideo && asset.thumbnail_url ? (
          <div className="relative w-full h-full">
            <img
              src={asset.thumbnail_url}
              alt={asset.name}
              className="w-full h-full object-cover"
              loading="lazy"
            />
            <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity">
              <Video className="h-8 w-8 text-accent" />
            </div>
          </div>
        ) : (
          <div className="text-muted-foreground">
            {typeIcons[type] || <FileText className="h-5 w-5" />}
          </div>
        )}

        {/* Type badge */}
        <span className="absolute top-2 left-2 px-1.5 py-0.5 text-[10px] font-mono uppercase bg-card/90 border border-border text-muted-foreground cyber-chamfer-xs">
          {asset.type}
        </span>

        {asset.status && asset.status !== "draft" && (
          <div className="absolute top-2 right-2">
            <StatusBadge status={asset.status} className="bg-card/90" />
          </div>
        )}

        {asset.duration_ms && (
          <span className="absolute bottom-2 right-2 px-1.5 py-0.5 text-[10px] font-mono bg-card/90 border border-border text-muted-foreground cyber-chamfer-xs">
            {formatDuration(asset.duration_ms)}
          </span>
        )}
      </div>

      {/* Info */}
      <div className="p-3">
        <p className="text-xs font-mono text-foreground truncate mb-1">
          {asset.name}
        </p>
        <div className="flex items-center justify-between text-[10px] text-muted-foreground">
          <span>{formatBytes(asset.size_bytes)}</span>
          <span>{formatDate(asset.created_at)}</span>
        </div>
        {onSubmitToFeed && (
          <button
            type="button"
            disabled={!canSubmit || submitLoading}
            onClick={(event) => {
              event.stopPropagation();
              onSubmitToFeed(asset);
            }}
            className="mt-3 inline-flex h-8 w-full items-center justify-center gap-1.5 border border-border text-[10px] font-mono uppercase tracking-wider text-muted-foreground cyber-chamfer-xs transition-colors hover:border-accent hover:text-accent disabled:pointer-events-none disabled:opacity-50"
          >
            <Send className="h-3.5 w-3.5" />
            {submitLabel}
          </button>
        )}
      </div>
    </div>
  );
}
