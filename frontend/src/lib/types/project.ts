export interface Project {
  id: string;
  user_id: string;
  name: string;
  description: string;
  cover_url: string;
  status: string;
  settings: unknown;
  created_at: string;
  updated_at: string;
}

export interface CreateProjectInput {
  name: string;
  description?: string;
}

export interface UpdateProjectInput {
  name?: string;
  description?: string;
  status?: string;
}
