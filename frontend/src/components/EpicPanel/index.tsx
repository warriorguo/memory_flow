import React, { useState } from 'react';
import { Alert, Button, Card, Input, Popconfirm, Progress, Space, Table, Typography, message } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { getEpic, setIssueEpic } from '../../api/epic';
import StatusTag from '../StatusTag';
import PriorityBadge from '../PriorityBadge';
import { ISSUE_STATUS_LABELS, ISSUE_TYPE_LABELS } from '../../types/common';
import type { IssueStatus, Priority } from '../../types/common';
import type { EpicChild, EpicCounts } from '../../types/epic';
import { statusBreakdown } from './format';

const { Text } = Typography;

type ApiError = { response?: { data?: { error?: string } } };

export const EpicProgressBar: React.FC<{ counts: EpicCounts; width?: number }> = ({ counts, width = 160 }) => (
  <Space size={8}>
    <Progress percent={Math.round(counts.done_ratio * 100)} size="small" style={{ width, margin: 0 }} />
    <Text type="secondary">{counts.done}/{counts.total}</Text>
  </Space>
);

/**
 * The epic view on an epic's detail page (MF-39): progress, the issues in it
 * (from any project), the dependencies among them and which are blocked.
 */
const EpicPanel: React.FC<{ epicKey: string }> = ({ epicKey }) => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [toAdd, setToAdd] = useState('');

  const { data: detail } = useQuery({
    queryKey: ['epic', epicKey],
    queryFn: () => getEpic(epicKey),
  });

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['epic'] });
    queryClient.invalidateQueries({ queryKey: ['epics'] });
    queryClient.invalidateQueries({ queryKey: ['issue'] });
    queryClient.invalidateQueries({ queryKey: ['issues'] });
  };

  const addMutation = useMutation({
    mutationFn: async (keys: string[]) => {
      const moved: string[] = [];
      for (const k of keys) {
        const res = await setIssueEpic(k, epicKey);
        if (res.previous && res.previous.issue_key !== epicKey) moved.push(`${k}（从 ${res.previous.issue_key} 移入）`);
      }
      return moved;
    },
    onSuccess: (moved) => {
      message.success(moved.length ? `已加入；${moved.join('、')}` : '已加入');
      setToAdd('');
      refresh();
    },
    onError: (err: ApiError) => message.error(err.response?.data?.error ?? '操作失败'),
  });

  const removeMutation = useMutation({
    mutationFn: (key: string) => setIssueEpic(key, ''),
    onSuccess: () => {
      message.success('已移出');
      refresh();
    },
    onError: (err: ApiError) => message.error(err.response?.data?.error ?? '操作失败'),
  });

  if (!detail) return null;
  const { counts } = detail;
  const add = () => {
    const keys = toAdd.split(/[\s,，]+/).map((k) => k.trim().toUpperCase()).filter(Boolean);
    if (keys.length) addMutation.mutate(keys);
  };

  const columns = [
    {
      title: '编号',
      dataIndex: 'issue_key',
      width: 110,
      render: (k: string) => <a onClick={() => navigate(`/issues/${k}`)}>{k}</a>,
    },
    { title: '标题', dataIndex: 'title' },
    { title: '类型', dataIndex: 'type', width: 70, render: (t: EpicChild['type']) => ISSUE_TYPE_LABELS[t] ?? t },
    { title: '优先级', dataIndex: 'priority', width: 100, render: (p: Priority) => <PriorityBadge priority={p} /> },
    { title: '状态', dataIndex: 'status', width: 90, render: (s: IssueStatus) => <StatusTag status={s} /> },
    { title: '负责人', dataIndex: 'assignee_id', width: 100, render: (a?: string) => a || '-' },
    {
      title: '',
      width: 60,
      render: (_: unknown, c: EpicChild) => (
        <Popconfirm title={`把 ${c.issue_key} 移出这个 Epic？`} onConfirm={() => removeMutation.mutate(c.issue_key)}>
          <a>移出</a>
        </Popconfirm>
      ),
    },
  ];

  return (
    <Card title="Epic 进度" style={{ marginTop: 16 }}>
      <Space direction="vertical" style={{ width: '100%' }} size={12}>
        <Space size={16} wrap>
          <EpicProgressBar counts={counts} width={240} />
          <Text type="secondary">{statusBreakdown(counts) || '还没有工作项'}</Text>
        </Space>

        {detail.blocked.length > 0 && (
          <Alert
            type="warning"
            showIcon
            message="以下工作项卡在依赖上（critical 依赖未完成）"
            description={
              <Space direction="vertical" size={2}>
                {detail.blocked.map((b) => (
                  <span key={b.issue.id}>
                    <a onClick={() => navigate(`/issues/${b.issue.issue_key}`)}>{b.issue.issue_key}</a> 在等{' '}
                    {b.blocked_by.map((r, i) => (
                      <span key={r.id}>
                        {i > 0 && '、'}
                        <a onClick={() => navigate(`/issues/${r.issue_key}`)}>{r.issue_key}</a>（{ISSUE_STATUS_LABELS[r.status as IssueStatus] ?? r.status}）
                      </span>
                    ))}
                  </span>
                ))}
              </Space>
            }
          />
        )}

        <Table rowKey="id" size="small" columns={columns} dataSource={detail.children} pagination={false} />

        {detail.dependencies.length > 0 && (
          <div>
            <Text strong>工作项之间的依赖</Text>
            <ul style={{ margin: '4px 0 0', paddingLeft: 20 }}>
              {detail.dependencies.map((d) => (
                <li key={`${d.source}-${d.target}-${d.type}`}>
                  {d.source} {d.type === 'blocks' ? '阻塞' : '依赖'} {d.target}
                  <Text type="secondary">（{d.severity}）</Text>
                </li>
              ))}
            </ul>
          </div>
        )}

        <Space.Compact style={{ width: 420, maxWidth: '100%' }}>
          <Input
            placeholder="加入工作项，如 OZX-12 OAE-3（可跨项目）"
            value={toAdd}
            onChange={(e) => setToAdd(e.target.value)}
            onPressEnter={add}
          />
          <Button type="primary" onClick={add} loading={addMutation.isPending}>
            加入
          </Button>
        </Space.Compact>
      </Space>
    </Card>
  );
};

export default EpicPanel;
