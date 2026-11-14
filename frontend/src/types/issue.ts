import type { IssueStatus, IssueType, Priority } from './common';
import type { IssueRef } from './dependency';

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
  // MF-36: set while status is "duplicate"; the refs come with a single-issue read.
  duplicate_of_id?: string;
  duplicate_of?: IssueRef;
  duplicates?: IssueRef[];
  // MF-39: the epic this issue belongs to; the ref comes with a single-issue read.
  epic_id?: string;
  epic?: IssueRef;
}

/** An existing issue that may be the same as a draft (MF-35). */
export interface SimilarIssue {
  id: string;
  key: string;
  title: string;
  type: IssueType;
  status: IssueStatus;
  score: number;
  level: 'likely' | 'possible';
  shared_identifiers?: string[];
  references?: boolean;
  title_similarity: number;
}

export interface SimilarIssuesRequest {
  type?: IssueType;
  title: string;
  description?: string;
  exclude_key?: string;
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
  // The filer saw the suspected duplicates and judged this a separate issue.
  not_duplicate?: boolean;
  // MF-39: file the issue into this epic (key or id).
  epic?: string;
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
  // Present: replaces the issue's tag set. Absent: tags stay as they are.
  tag_ids?: string[];
}

export interface IssueFilter {
  type?: IssueType;
  status?: IssueStatus;
  priority?: Priority;
  assignee_id?: string;
  creator_id?: string;
  // Tag ids or names; an issue must carry every one (AND).
  tag?: string[];
  keyword?: string;
  // MF-39: only the issues of this epic (key or id).
  epic?: string;
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
