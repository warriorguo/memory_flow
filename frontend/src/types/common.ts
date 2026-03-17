export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface ApiResponse<T> {
  data: T;
}

export type ProjectStatus = 'active' | 'paused' | 'archived';
export type IssueType = 'requirement' | 'bug';
export type Priority = 'P0' | 'P1' | 'P2';
export type IssueStatus = 'todo' | 'in_progress' | 'review' | 'testing' | 'done' | 'closed' | 'rejected';
export type MemoryType = 'recall' | 'write';

export const ISSUE_STATUS_LABELS: Record<IssueStatus, string> = {
  todo: '待处理',
  in_progress: '进行中',
  review: '评审中',
  testing: '测试中',
  done: '已完成',
  closed: '已关闭',
  rejected: '已拒绝',
};

export const PRIORITY_LABELS: Record<Priority, string> = {
  P0: 'P0 - 紧急',
  P1: 'P1 - 重要',
  P2: 'P2 - 一般',
};

export const STATUS_COLORS: Record<IssueStatus, string> = {
  todo: 'default',
  in_progress: 'processing',
  review: 'warning',
  testing: 'purple',
  done: 'success',
  closed: 'default',
  rejected: 'error',
};

export const PRIORITY_COLORS: Record<Priority, string> = {
  P0: 'red',
  P1: 'orange',
  P2: 'blue',
};

// Allowed status transitions
export const ALLOWED_TRANSITIONS: Record<IssueStatus, IssueStatus[]> = {
  todo: ['in_progress', 'rejected'],
  in_progress: ['review', 'todo'],
  review: ['testing', 'in_progress'],
  testing: ['done', 'in_progress'],
  done: ['closed', 'in_progress'],
  closed: [],
  rejected: ['todo'],
};
