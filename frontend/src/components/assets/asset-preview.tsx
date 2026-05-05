"use client";

import { Dialog } from "@/components/ui/dialog";
import type { Asset } from "@/lib/types/asset";
import { formatBytes, formatDuration } from "@/lib/utils/format";
import { Video, Music, Image } from "lucide-react";

interface AssetPreviewProps {
  asset: Asset | null;
  open: boolean;
  onClose: () => void;
}

export function AssetPreview({ asset, open, onClose }: AssetPreviewProps) {
  if (!asset) return null;

  return (
    <Dialog open={open} onClose={onClose} title={asset.name} className="max-w-3xl">
      <div className="space-y-4">
        {/* Preview */}
        <div className="aspect-video bg-muted flex items-center justify-center overflow-hidden cyber-chamfer-sm">
          {asset.type === "image" || asset.mime_type.startsWith("image/") ? (
            <img
              src={asset.url}
              alt={asset.name}
              className="w-full h-full object-contain"
            />
          ) : asset.type === "video" || asset.mime_type.startsWith("video/") ? (
            <video
              src={asset.url}
              controls
              className="w-full h-full"
              poster={asset.thumbnail_url}
            >
              Your browser does not support video playback.
            </video>
          ) : asset.type === "audio" || asset.mime_type.startsWith("audio/") ? (
            <div className="flex flex-col items-center gap-4">
              <Music className="h-16 w-16 text-accent" />
              <audio src={asset.url} controls className="w-80" />
            </div>
          ) : (
            <Image className="h-16 w-16 text-muted-foreground" />
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
