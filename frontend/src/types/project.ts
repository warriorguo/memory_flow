export interface Project {
  id: string;
  key: string;
  name: string;
  summary?: string;
  description?: string;
  design_principles?: string;
  git_url?: string;
  cicd_url?: string;
  doc_url?: string;
  owner_id?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface CreateProjectRequest {
  key: string;
  name: string;
  summary?: string;
  description?: string;
  design_principles?: string;
  git_url?: string;
  cicd_url?: string;
  doc_url?: string;
  owner_id?: string;
}

export interface UpdateProjectRequest extends Partial<CreateProjectRequest> {
  status?: string;
}

export interface ProjectFilter {
  name?: string;
  status?: string;
  owner_id?: string;
  page?: number;
  page_size?: number;
}
