import { ISSUE_STATUS_LABELS } from '../../types/common';
import type { IssueStatus } from '../../types/common';
import type { EpicCounts } from '../../types/epic';

/** One-line status breakdown, e.g. "待处理 2 · 已完成 1". */
export const statusBreakdown = (counts: EpicCounts): string =>
  Object.entries(counts.status_counts)
    .map(([s, n]) => `${ISSUE_STATUS_LABELS[s as IssueStatus] ?? s} ${n}`)
    .join(' · ');
