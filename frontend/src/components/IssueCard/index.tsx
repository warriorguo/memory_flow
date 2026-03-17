import React from 'react';
import { Card, Typography } from 'antd';
import StatusTag from '../StatusTag';
import PriorityBadge from '../PriorityBadge';
import type { Issue } from '../../types/issue';

const { Text } = Typography;

interface IssueCardProps {
  issue: Issue;
  onClick?: () => void;
}

const IssueCard: React.FC<IssueCardProps> = ({ issue, onClick }) => (
  <Card
    size="small"
    hoverable
    onClick={onClick}
    style={{ marginBottom: 8 }}
  >
    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
      <Text type="secondary" style={{ fontSize: 12 }}>{issue.issue_key}</Text>
      <PriorityBadge priority={issue.priority} />
    </div>
    <Text strong style={{ display: 'block', marginBottom: 4 }}>{issue.title}</Text>
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
      <StatusTag status={issue.status} />
      {issue.assignee_id && <Text type="secondary" style={{ fontSize: 12 }}>{issue.assignee_id}</Text>}
    </div>
  </Card>
);

export default IssueCard;
