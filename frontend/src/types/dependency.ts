import type { IssueStatus, Priority } from './common';

/** The summary of an issue at one end of a dependency. */
export interface IssueRef {
  id: string;
  issue_key: string;
  title: string;
  status: IssueStatus;
  priority: Priority;
}

export type DependencyType = 'depends_on' | 'blocks';
export type DependencySeverity = 'critical' | 'recommended';

/**
 * One stored dependency. It reads source → target: with type depends_on the
 * source needs the target; with blocks the source holds the target up.
 */
export interface IssueDependency {
  id: string;
  source_issue_id: string;
  target_issue_id: string;
  type: DependencyType;
  severity: DependencySeverity;
  created_at: string;
  source_issue?: IssueRef;
  target_issue?: IssueRef;
}
