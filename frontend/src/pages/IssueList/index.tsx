import React, { useState } from 'react';
import { Table, Button, Input, Select, Space, Card, Modal, Form, message } from 'antd';
import { PlusOutlined, SearchOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router-dom';
import { listIssues, createIssue } from '../../api/issue';
import StatusTag from '../../components/StatusTag';
import PriorityBadge from '../../components/PriorityBadge';
import TagSelector from '../../components/TagSelector';
import type { IssueFilter, CreateIssueRequest, Issue } from '../../types/issue';
import type { IssueStatus, IssueType, Priority } from '../../types/common';
import { ISSUE_STATUS_LABELS, PRIORITY_LABELS } from '../../types/common';
import dayjs from 'dayjs';

interface Props {
  projectId?: string;
}

const IssueListView: React.FC<Props> = ({ projectId: propProjectId }) => {
  const params = useParams<{ projectId: string }>();
  const projectId = propProjectId || params.projectId!;
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState<IssueFilter>({ page: 1, page_size: 20 });
  const [createOpen, setCreateOpen] = useState(false);
  const [form] = Form.useForm();

  const { data, isLoading } = useQuery({
    queryKey: ['issues', projectId, filter],
    queryFn: () => listIssues(projectId, filter),
    enabled: !!projectId,
  });

  const createMutation = useMutation({
    mutationFn: (req: CreateIssueRequest) => createIssue(projectId, req),
    onSuccess: () => {
      message.success('创建成功');
      queryClient.invalidateQueries({ queryKey: ['issues', projectId] });
      setCreateOpen(false);
      form.resetFields();
    },
    onError: () => message.error('操作失败'),
  });

  const columns = [
    { title: '编号', dataIndex: 'issue_key', width: 120 },
    {
      title: '标题',
      dataIndex: 'title',
      render: (title: string, record: Issue) => (
        <a onClick={() => navigate(`/issues/${record.id}`)}>{title}</a>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 80,
      render: (t: string) => t === 'requirement' ? '需求' : '缺陷',
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      width: 120,
      render: (p: Priority) => <PriorityBadge priority={p} />,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (s: IssueStatus) => <StatusTag status={s} />,
    },
    { title: '指派人', dataIndex: 'assignee_id', width: 100 },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      width: 160,
      render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm'),
    },
  ];

  return (
    <Card>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Space wrap>
          <Input
            placeholder="搜索"
            prefix={<SearchOutlined />}
            allowClear
            onChange={(e) => setFilter((f) => ({ ...f, keyword: e.target.value || undefined, page: 1 }))}
            style={{ width: 160 }}
          />
          <Select
            placeholder="类型"
            allowClear
            style={{ width: 100 }}
            onChange={(v) => setFilter((f) => ({ ...f, type: v as IssueType, page: 1 }))}
            options={[
              { label: '需求', value: 'requirement' },
              { label: '缺陷', value: 'bug' },
            ]}
          />
          <Select
            placeholder="状态"
            allowClear
            style={{ width: 120 }}
            onChange={(v) => setFilter((f) => ({ ...f, status: v as IssueStatus, page: 1 }))}
            options={Object.entries(ISSUE_STATUS_LABELS).map(([k, v]) => ({ label: v, value: k }))}
          />
          <Select
            placeholder="优先级"
            allowClear
            style={{ width: 130 }}
            onChange={(v) => setFilter((f) => ({ ...f, priority: v as Priority, page: 1 }))}
            options={Object.entries(PRIORITY_LABELS).map(([k, v]) => ({ label: v, value: k }))}
          />
        </Space>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          创建工作项
        </Button>
      </div>
      <Table
        rowKey="id"
        columns={columns}
        dataSource={data?.data}
        loading={isLoading}
        pagination={{
          current: filter.page,
          pageSize: filter.page_size,
          total: data?.total,
          onChange: (page, pageSize) => setFilter((f) => ({ ...f, page, page_size: pageSize })),
        }}
      />

      <Modal
        title="创建工作项"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={createMutation.isPending}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={(v) => createMutation.mutate(v)} initialValues={{ type: 'requirement', priority: 'P2' }}>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select options={[{ label: '需求', value: 'requirement' }, { label: '缺陷', value: 'bug' }]} />
          </Form.Item>
          <Form.Item name="title" label="标题" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={4} />
          </Form.Item>
          <Form.Item name="priority" label="优先级">
            <Select options={Object.entries(PRIORITY_LABELS).map(([k, v]) => ({ label: v, value: k }))} />
          </Form.Item>
          <Form.Item name="assignee_id" label="指派人">
            <Input />
          </Form.Item>
          <Form.Item name="source" label="来源">
            <Input />
          </Form.Item>
          <Form.Item name="version" label="关联版本">
            <Input />
          </Form.Item>
          <Form.Item name="tag_ids" label="标签">
            <TagSelector />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
};

export default IssueListView;
