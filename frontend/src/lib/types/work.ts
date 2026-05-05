export interface Work {
  id: string;
  project_id: string;
  user_id: string;
  title: string;
  description: string;
  duration_ms: number | null;
  resolution: string | null;
  format: string | null;
  file_url: string;
  thumbnail_url: string;
  status: string;
  metadata: unknown;
  published_at: string | null;
  created_at: string;
  updated_at: string;
}
