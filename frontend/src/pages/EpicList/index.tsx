import React, { useState } from 'react';
import { Button, Card, Form, Input, Modal, Select, Switch, Space, Table, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { listEpics } from '../../api/epic';
import { createIssue } from '../../api/issue';
import StatusTag from '../../components/StatusTag';
import PriorityBadge from '../../components/PriorityBadge';
import { EpicProgressBar } from '../../components/EpicPanel';
import { statusBreakdown } from '../../components/EpicPanel/format';
import { PRIORITY_LABELS } from '../../types/common';
import type { IssueStatus, Priority } from '../../types/common';
import type { EpicSummary } from '../../types/epic';
import dayjs from 'dayjs';

const CLOSED: IssueStatus[] = ['done', 'closed', 'rejected', 'duplicate'];

/** A project's epics with how far along each one is (MF-39). */
const EpicListView: React.FC<{ projectId: string }> = ({ projectId }) => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [showClosed, setShowClosed] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [form] = Form.useForm();

  const { data: epics = [], isLoading } = useQuery({
    queryKey: ['epics', projectId],
    queryFn: () => listEpics(projectId),
    enabled: !!projectId,
  });

  const createMutation = useMutation({
    mutationFn: (v: { title: string; description?: string; priority?: Priority }) =>
      createIssue(projectId, { ...v, type: 'epic' }),
    onSuccess: (epic) => {
      message.success(`已创建 ${epic.issue_key}`);
      queryClient.invalidateQueries({ queryKey: ['epics', projectId] });
      queryClient.invalidateQueries({ queryKey: ['issues', projectId] });
      setCreateOpen(false);
      form.resetFields();
    },
    onError: () => message.error('创建失败'),
  });

  const shown = showClosed ? epics : epics.filter((e) => !CLOSED.includes(e.status));

  const columns = [
    {
      title: '编号',
      dataIndex: 'issue_key',
      width: 110,
      render: (k: string) => <a onClick={() => navigate(`/issues/${k}`)}>{k}</a>,
    },
    { title: '标题', dataIndex: 'title', render: (t: string, e: EpicSummary) => <a onClick={() => navigate(`/issues/${e.issue_key}`)}>{t}</a> },
    { title: '状态', dataIndex: 'status', width: 90, render: (s: IssueStatus) => <StatusTag status={s} /> },
    { title: '优先级', dataIndex: 'priority', width: 100, render: (p: Priority) => <PriorityBadge priority={p} /> },
    { title: '完成度', width: 240, render: (_: unknown, e: EpicSummary) => <EpicProgressBar counts={e.counts} /> },
    { title: '分布', render: (_: unknown, e: EpicSummary) => statusBreakdown(e.counts) || '-' },
    { title: '更新时间', dataIndex: 'updated_at', width: 150, render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm') },
  ];

  return (
    <Card>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Space>
          <Switch checked={showClosed} onChange={setShowClosed} />
          <span>显示已完成 / 已关闭</span>
        </Space>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          创建 Epic
        </Button>
      </div>
      <Table rowKey="id" columns={columns} dataSource={shown} loading={isLoading} pagination={false}
        locale={{ emptyText: epics.length ? '没有进行中的 Epic' : '还没有 Epic' }} />

      <Modal
        title="创建 Epic"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={createMutation.isPending}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={(v) => createMutation.mutate(v)} initialValues={{ priority: 'P1' }}>
          <Form.Item name="title" label="标题" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label="描述（目标、范围、拆分思路）">
            <Input.TextArea rows={5} />
          </Form.Item>
          <Form.Item name="priority" label="优先级">
            <Select options={Object.entries(PRIORITY_LABELS).map(([k, v]) => ({ label: v, value: k }))} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
};

export default EpicListView;
