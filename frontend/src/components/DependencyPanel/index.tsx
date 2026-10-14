import React from 'react';
import { Card, List, Space, Tag, Typography } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { listDependencies } from '../../api/dependency';
import type { IssueDependency, IssueRef } from '../../types/dependency';
import StatusTag from '../StatusTag';

interface DependencyPanelProps {
  /** Issue key (OZX-12) or UUID — the API accepts either. */
  issueKey: string;
  /** The issue's UUID, to tell which end of each dependency it is. */
  issueId: string;
}

interface Related {
  dep: IssueDependency;
  other: IssueRef;
}

/**
 * relate restates a stored dependency from this issue's side. It is stored as
 * source → target, and either end may be this issue: "A depends_on B" and
 * "B blocks A" both mean A waits for B.
 */
function relate(dep: IssueDependency, issueId: string): { waitsFor: boolean; other?: IssueRef } {
  const isSource = dep.source_issue_id === issueId;
  const other = isSource ? dep.target_issue : dep.source_issue;
  const waitsFor = isSource ? dep.type === 'depends_on' : dep.type === 'blocks';
  return { waitsFor, other };
}

const SEVERITY_LABELS = { critical: '强依赖', recommended: '弱关联' } as const;

const RelatedList: React.FC<{ title: string; items: Related[] }> = ({ title, items }) => {
  if (items.length === 0) return null;
  return (
    <>
      <Typography.Text type="secondary">{title}</Typography.Text>
      <List
        size="small"
        dataSource={items}
        renderItem={({ dep, other }) => (
          <List.Item key={dep.id}>
            <Space wrap>
              <Link to={`/issues/${other.issue_key}`}>{other.issue_key}</Link>
              <span>{other.title}</span>
              <StatusTag status={other.status} />
              <Tag color={dep.severity === 'critical' ? 'red' : 'default'}>{SEVERITY_LABELS[dep.severity]}</Tag>
            </Space>
          </List.Item>
        )}
      />
    </>
  );
};

/**
 * DependencyPanel shows which issues this one waits for and which wait for it,
 * across projects. It renders nothing when the issue has no dependencies.
 */
const DependencyPanel: React.FC<DependencyPanelProps> = ({ issueKey, issueId }) => {
  const { data: deps = [] } = useQuery({
    queryKey: ['dependencies', issueKey],
    queryFn: () => listDependencies(issueKey),
  });

  const waitsFor: Related[] = [];
  const blocking: Related[] = [];
  for (const dep of deps) {
    const { waitsFor: wait, other } = relate(dep, issueId);
    // A server older than MF-11 sends no summaries; there is nothing to link to.
    if (!other) continue;
    (wait ? waitsFor : blocking).push({ dep, other });
  }
  if (waitsFor.length + blocking.length === 0) return null;

  return (
    <Card title="依赖关系" style={{ marginTop: 16 }}>
      <RelatedList title="依赖于（需要先完成）" items={waitsFor} />
      <RelatedList title="阻塞（在等本项完成）" items={blocking} />
    </Card>
  );
};

export default DependencyPanel;
