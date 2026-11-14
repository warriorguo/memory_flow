import type { Issue } from './issue';
import type { IssueRef } from './dependency';
import type { Memory } from './memory';
import type { TrendData } from './issue';

/** How far along an epic is. Rejected and duplicate issues are not counted. */
export interface EpicCounts {
  total: number;
  done: number;
  done_ratio: number;
  status_counts: Record<string, number>;
}

export interface EpicSummary extends Issue {
  counts: EpicCounts;
}

export interface EpicChild extends Issue {
  project_key: string;
}

export interface EpicDependency {
  source: string;
  target: string;
  type: 'depends_on' | 'blocks';
  severity: 'critical' | 'recommended';
}

export interface BlockedIssue {
  issue: IssueRef;
  blocked_by: IssueRef[];
}

export interface EpicDetail {
  epic: Issue;
  children: EpicChild[];
  counts: EpicCounts;
  dependencies: EpicDependency[];
  blocked: BlockedIssue[];
}

export interface EpicProgress {
  counts: EpicCounts;
  priority_counts: Record<string, number>;
  type_counts: Record<string, number>;
  trend: TrendData[];
  blocked: BlockedIssue[];
}

/** A memory as seen from one issue: its own, or shared from its epic. */
export interface IssueMemory extends Memory {
  scope: 'issue' | 'epic';
  epic_key?: string;
  revision: string;
  unread: boolean;
  updated: boolean;
}

export interface SetEpicResult {
  issue: Issue;
  previous?: IssueRef;
}
