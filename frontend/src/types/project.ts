// One CI/CD pipeline the project deploys through; label tells several apart.
export interface CICDLink {
  label: string;
  url: string;
}

export interface Project {
  id: string;
  key: string;
  name: string;
  summary?: string;
  description?: string;
  design_principles?: string;
  git_url?: string;
  /** Legacy single link: mirrors cicd_links[0].url. */
  cicd_url?: string;
  cicd_links: CICDLink[];
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
  cicd_links?: CICDLink[];
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
