"use client";

import { useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { CyberInput } from "@/components/ui/input";
import { CyberButton } from "@/components/ui/button";
import { assetsApi } from "@/lib/api/assets";
import { useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/lib/hooks/query-keys";
import { Upload } from "lucide-react";

interface AssetUploadDialogProps {
  open: boolean;
  onClose: () => void;
  projectId: string;
}

export function AssetUploadDialog({ open, onClose, projectId }: AssetUploadDialogProps) {
  const [name, setName] = useState("");
  const [mimeType, setMimeType] = useState("image/png");
  const [type, setType] = useState("image");
  const [sizeBytes, setSizeBytes] = useState("");
  const [uploading, setUploading] = useState(false);
  const [uploadUrl, setUploadUrl] = useState<string | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const queryClient = useQueryClient();

  const handleRequestUrl = async () => {
    setUploading(true);
    try {
      const result = await assetsApi.getUploadUrl({
        project_id: projectId,
        type,
        name: name || "untitled",
        mime_type: mimeType,
        size_bytes: sizeBytes ? parseInt(sizeBytes) : undefined,
      });
      setUploadUrl(result.upload_url);
    } catch (err) {
      console.error("Failed to get upload URL", err);
    } finally {
      setUploading(false);
    }
  };

  const handleFileUpload = async () => {
    if (!selectedFile || !uploadUrl) return;
    setUploading(true);
    try {
      await fetch(uploadUrl, {
        method: "PUT",
        body: selectedFile,
        headers: { "Content-Type": mimeType },
      });
      queryClient.invalidateQueries({ queryKey: queryKeys.projects.assets(projectId, 1) });
      onClose();
      setName("");
      setUploadUrl(null);
      setSelectedFile(null);
    } catch (err) {
      console.error("Upload failed", err);
    } finally {
      setUploading(false);
    }
  };

  return (
    <Dialog open={open} onClose={onClose} title="Upload Asset">
      <div className="space-y-4">
        <CyberInput
          label="Asset Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="my-asset"
        />
        <div className="grid grid-cols-2 gap-3">
          <CyberInput
            label="Type"
            value={type}
            onChange={(e) => setType(e.target.value)}
            placeholder="image"
          />
          <CyberInput
            label="MIME Type"
            value={mimeType}
            onChange={(e) => setMimeType(e.target.value)}
            placeholder="image/png"
          />
        </div>
        <CyberInput
          label="Size (bytes)"
          value={sizeBytes}
          onChange={(e) => setSizeBytes(e.target.value)}
          placeholder="102400"
        />

        {!uploadUrl ? (
          <CyberButton
            variant="glitch"
            onClick={handleRequestUrl}
            loading={uploading}
            className="w-full"
          >
            Request Upload URL
          </CyberButton>
        ) : (
          <div className="space-y-3">
            <p className="text-xs font-mono text-accent">Upload URL ready</p>
            <input
              type="file"
              onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
              className="w-full text-xs font-mono text-muted-foreground file:bg-card file:text-accent file:border file:border-border file:px-3 file:py-1 file:mr-3 file:font-mono cursor-pointer"
            />
            <CyberButton
              variant="glitch"
              onClick={handleFileUpload}
              loading={uploading}
              disabled={!selectedFile}
              className="w-full"
            >
              <Upload className="h-4 w-4 mr-1" />
              Upload File
            </CyberButton>
          </div>
        )}
      </div>
    </Dialog>
  );
}
