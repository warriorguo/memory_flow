import type { IssueStatus, IssueType, Priority } from './common';

export interface Issue {
  id: string;
  issue_key: string;
  project_id: string;
  type: IssueType;
  title: string;
  description?: string;
  priority: Priority;
  status: IssueStatus;
  assignee_id?: string;
  creator_id?: string;
  source?: string;
  version?: string;
  git_url?: string;
  pr_url?: string;
  doc_url?: string;
  tags?: Tag[];
  created_at: string;
  updated_at: string;
}

export interface Tag {
  id: string;
  name: string;
  color?: string;
}

export interface IssueHistory {
  id: string;
  issue_id: string;
  field_name: string;
  old_value?: string;
  new_value?: string;
  operator_id?: string;
  created_at: string;
}

export interface CreateIssueRequest {
  type: IssueType;
  title: string;
  description?: string;
  priority?: Priority;
  assignee_id?: string;
  source?: string;
  version?: string;
  git_url?: string;
  pr_url?: string;
  doc_url?: string;
  tag_ids?: string[];
}

export interface UpdateIssueRequest {
  title?: string;
  description?: string;
  priority?: Priority;
  assignee_id?: string;
  source?: string;
  version?: string;
  git_url?: string;
  pr_url?: string;
  doc_url?: string;
}

export interface IssueFilter {
  type?: IssueType;
  status?: IssueStatus;
  priority?: Priority;
  assignee_id?: string;
  creator_id?: string;
  tag?: string;
  keyword?: string;
  page?: number;
  page_size?: number;
}

export interface ProgressSummary {
  status_counts: Record<string, number>;
  priority_counts: Record<string, number>;
  type_counts: Record<string, number>;
  total: number;
}

export interface TrendData {
  date: string;
  created: number;
  done: number;
}
