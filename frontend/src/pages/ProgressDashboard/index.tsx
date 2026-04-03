import React from 'react';
import { Card, Row, Col, Statistic, Spin } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis, Tooltip, Legend, LineChart, Line, ResponsiveContainer } from 'recharts';
import { getProgressSummary, getProgressTrend } from '../../api/issue';
import { useParams } from 'react-router-dom';
import { ISSUE_STATUS_LABELS } from '../../types/common';
import type { IssueStatus } from '../../types/common';


interface Props {
  projectId?: string;
}

const STATUS_CHART_COLORS: Record<string, string> = {
  todo: '#d9d9d9',
  in_progress: '#1890ff',
  review: '#faad14',
  testing: '#722ed1',
  done: '#52c41a',
  closed: '#8c8c8c',
  rejected: '#ff4d4f',
};

const PRIORITY_CHART_COLORS: Record<string, string> = {
  P0: '#ff4d4f',
  P1: '#fa8c16',
  P2: '#1890ff',
};

const ProgressDashboard: React.FC<Props> = ({ projectId: propProjectId }) => {
  const params = useParams<{ projectId: string }>();
  const projectId = propProjectId || params.projectId!;

  const { data: summary, isLoading: summaryLoading } = useQuery({
    queryKey: ['progress-summary', projectId],
    queryFn: () => getProgressSummary(projectId),
    enabled: !!projectId,
  });

  const { data: trend = [], isLoading: trendLoading } = useQuery({
    queryKey: ['progress-trend', projectId],
    queryFn: () => getProgressTrend(projectId, 30),
    enabled: !!projectId,
  });

  if (summaryLoading) return <Spin />;
  if (!summary) return null;

  const statusData = Object.entries(summary.status_counts).map(([key, value]) => ({
    name: ISSUE_STATUS_LABELS[key as IssueStatus] || key,
    value,
    color: STATUS_CHART_COLORS[key] || '#999',
  }));

  const priorityData = Object.entries(summary.priority_counts).map(([key, value]) => ({
    name: key,
    value,
    color: PRIORITY_CHART_COLORS[key] || '#999',
  }));

  const doneCount = summary.status_counts['done'] || 0;
  const inProgressCount = summary.status_counts['in_progress'] || 0;
  const todoCount = summary.status_counts['todo'] || 0;

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}><Card><Statistic title="总工作项" value={summary.total} /></Card></Col>
        <Col span={6}><Card><Statistic title="已完成" value={doneCount} valueStyle={{ color: '#52c41a' }} /></Card></Col>
        <Col span={6}><Card><Statistic title="进行中" value={inProgressCount} valueStyle={{ color: '#1890ff' }} /></Card></Col>
        <Col span={6}><Card><Statistic title="待处理" value={todoCount} /></Card></Col>
      </Row>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={12}>
          <Card title="状态分布">
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie data={statusData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={100} label>
                  {statusData.map((entry, i) => <Cell key={i} fill={entry.color} />)}
                </Pie>
                <Tooltip />
                <Legend />
              </PieChart>
            </ResponsiveContainer>
          </Card>
        </Col>
        <Col span={12}>
          <Card title="优先级分布">
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={priorityData}>
                <XAxis dataKey="name" />
                <YAxis />
                <Tooltip />
                <Bar dataKey="value">
                  {priorityData.map((entry, i) => <Cell key={i} fill={entry.color} />)}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </Card>
        </Col>
      </Row>

      {!trendLoading && trend.length > 0 && (
        <Card title="趋势 (近30天)" style={{ marginBottom: 24 }}>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={trend}>
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip />
              <Legend />
              <Line type="monotone" dataKey="created" stroke="#1890ff" name="新增" />
              <Line type="monotone" dataKey="done" stroke="#52c41a" name="完成" />
            </LineChart>
          </ResponsiveContainer>
        </Card>
      )}

    </div>
  );
};

export default ProgressDashboard;
