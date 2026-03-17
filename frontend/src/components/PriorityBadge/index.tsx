import React from 'react';
import { Tag } from 'antd';
import { PRIORITY_LABELS, PRIORITY_COLORS } from '../../types/common';
import type { Priority } from '../../types/common';

interface PriorityBadgeProps {
  priority: Priority;
}

const PriorityBadge: React.FC<PriorityBadgeProps> = ({ priority }) => (
  <Tag color={PRIORITY_COLORS[priority]}>{PRIORITY_LABELS[priority]}</Tag>
);

export default PriorityBadge;
