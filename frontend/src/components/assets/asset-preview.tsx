"use client";

import { useQuery } from "@tanstack/react-query";
import { Dialog } from "@/components/ui/dialog";
import type { Asset } from "@/lib/types/asset";
import { formatBytes, formatDuration } from "@/lib/utils/format";
import { Music, Image as ImageIcon, FileText, Loader2 } from "lucide-react";

interface AssetPreviewProps {
  asset: Asset | null;
  open: boolean;
  onClose: () => void;
}

export function AssetPreview({ asset, open, onClose }: AssetPreviewProps) {
  const type = asset?.type.toLowerCase() || "";
  const mimeType = asset?.mime_type.toLowerCase() || "";
  const isImage = type === "image" || mimeType.startsWith("image/");
  const isVideo = type === "video" || mimeType.startsWith("video/");
  const isAudio = type === "audio" || mimeType.startsWith("audio/");
  const isText =
    type === "text" ||
    mimeType.startsWith("text/") ||
    mimeType.includes("json") ||
    mimeType.includes("xml");
  const textQuery = useQuery({
    queryKey: ["asset-text-preview", asset?.id, asset?.url],
    enabled: open && !!asset?.url && isText,
    queryFn: async () => {
      if (!asset?.url) return "";
      const response = await fetch(asset.url);
      if (!response.ok) {
        throw new Error(`Failed to fetch text asset: ${response.status}`);
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

  if (!asset) return null;

  return (
    <Dialog open={open} onClose={onClose} title={asset.name} className="max-w-3xl">
      <div className="space-y-4">
        {/* Preview */}
        <div className="aspect-video bg-muted flex items-center justify-center overflow-hidden cyber-chamfer-sm">
          {isText ? (
            <div className="h-full w-full p-3">
              {textQuery.isLoading ? (
                <div className="flex h-full items-center justify-center text-muted-foreground">
                  <Loader2 className="h-5 w-5 animate-spin" />
                </div>
              ) : textQuery.isError ? (
                <div className="flex h-full flex-col items-center justify-center gap-2 text-muted-foreground">
                  <FileText className="h-10 w-10" />
                  <p className="text-xs font-mono">Unable to load text content.</p>
                </div>
              ) : (
                <textarea
                  readOnly
                  value={textQuery.data || ""}
                  className="h-full w-full resize-none bg-background/80 border border-border p-3 text-xs font-mono leading-relaxed text-foreground outline-none cyber-chamfer-xs"
                />
              )}
            </div>
          ) : isImage ? (
            <img
              src={asset.url}
              alt={asset.name}
              className="w-full h-full object-contain"
            />
          ) : isVideo ? (
            <video
              src={asset.url}
              controls
              className="w-full h-full"
              poster={asset.thumbnail_url}
            >
              Your browser does not support video playback.
            </video>
          ) : isAudio ? (
            <div className="flex flex-col items-center gap-4">
              <Music className="h-16 w-16 text-accent" />
              <audio src={asset.url} controls className="w-80" />
            </div>
          ) : (
            <ImageIcon className="h-16 w-16 text-muted-foreground" />
          )}
        </div>

        {/* Metadata */}
        <div className="grid grid-cols-2 gap-2 text-xs font-mono bg-muted/30 p-3 cyber-chamfer-xs">
          <span className="text-muted-foreground">Type:</span>
          <span className="text-foreground uppercase">{asset.type}</span>
          <span className="text-muted-foreground">MIME:</span>
          <span className="text-foreground">{asset.mime_type}</span>
          <span className="text-muted-foreground">Size:</span>
          <span className="text-accent">{formatBytes(asset.size_bytes)}</span>
          {asset.duration_ms && (
            <>
              <span className="text-muted-foreground">Duration:</span>
              <span className="text-accent">{formatDuration(asset.duration_ms)}</span>
            </>
          )}
          {asset.width && asset.height && (
            <>
              <span className="text-muted-foreground">Resolution:</span>
              <span className="text-accent">{asset.width}x{asset.height}</span>
            </>
          )}
        </div>
      </div>
    </Dialog>
  );
}
