import React from 'react';
import { Tag } from 'antd';
import { ISSUE_STATUS_LABELS, STATUS_COLORS } from '../../types/common';
import type { IssueStatus } from '../../types/common';

interface StatusTagProps {
  status: IssueStatus;
}

const StatusTag: React.FC<StatusTagProps> = ({ status }) => (
  <Tag color={STATUS_COLORS[status]}>{ISSUE_STATUS_LABELS[status]}</Tag>
);

export default StatusTag;
