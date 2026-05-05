export interface Asset {
  id: string;
  user_id: string;
  project_id: string | null;
  type: string;
  name: string;
  mime_type: string;
  size_bytes: number;
  duration_ms: number | null;
  width: number | null;
  height: number | null;
  checksum: string;
  storage_key: string;
  storage_bucket: string;
  url: string;
  thumbnail_url: string;
  metadata: unknown;
  created_at: string;
}

export interface UploadUrlResponse {
  asset: Asset;
  upload_url: string;
  expires_in: number;
  method: string;
  content_type: string;
}

export interface UploadUrlInput {
  project_id: string;
  type: string;
  name: string;
  mime_type: string;
  size_bytes?: number;
  duration_ms?: number;
  width?: number;
  height?: number;
  checksum?: string;
}

export interface ComposeInput {
  image_asset_ids: string[];
  subtitle?: string;
  subtitle_format?: "srt" | "vtt";
  duration_per_image_ms?: number;
  width?: number;
  height?: number;
  fps?: number;
  title?: string;
  idempotency_key?: string;
}

export interface ComposeResult {
  task_id: string;
  video_asset_id: string;
  cover_asset_id: string;
  subtitle_asset_id?: string;
  video_url: string;
  cover_url: string;
  duration_ms: number;
}
