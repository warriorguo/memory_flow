import React, { useState } from 'react';
import { Table, Button, Input, Select, Space, Card, Modal, Form, message } from 'antd';
import { PlusOutlined, SearchOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { listProjects, createProject, archiveProject } from '../../api/project';
import type { ProjectFilter, CreateProjectRequest } from '../../types/project';
import dayjs from 'dayjs';

const ProjectList: React.FC = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState<ProjectFilter>({ page: 1, page_size: 20 });
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [form] = Form.useForm();

  const { data, isLoading } = useQuery({
    queryKey: ['projects', filter],
    queryFn: () => listProjects(filter),
  });

  const createMutation = useMutation({
    mutationFn: (req: CreateProjectRequest) => createProject(req),
    onSuccess: () => {
      message.success('项目创建成功');
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setCreateModalOpen(false);
      form.resetFields();
    },
    onError: () => message.error('操作失败'),
  });

  const archiveMutation = useMutation({
    mutationFn: (id: string) => archiveProject(id),
    onSuccess: () => {
      message.success('项目已归档');
      queryClient.invalidateQueries({ queryKey: ['projects'] });
    },
    onError: () => message.error('操作失败'),
  });

  const columns = [
    { title: '项目标识', dataIndex: 'key', width: 100 },
    {
      title: '项目名称',
      dataIndex: 'name',
      render: (name: string, record: any) => (
        <a onClick={() => navigate(`/projects/${record.key}`)}>{name}</a>
      ),
    },
    { title: '负责人', dataIndex: 'owner_id', width: 120 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (status: string) => status === 'active' ? '进行中' : status === 'paused' ? '暂停' : '已归档',
    },
    { title: 'Git', dataIndex: 'git_url', width: 200, ellipsis: true },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      width: 180,
      render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '操作',
      width: 100,
      render: (_: any, record: any) => (
        <Button
          type="link"
          danger
          size="small"
          onClick={() => {
            Modal.confirm({
              title: '确认归档该项目？',
              onOk: () => archiveMutation.mutate(record.id),
            });
          }}
        >
          归档
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Card>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
          <Space>
            <Input
              placeholder="搜索项目名称"
              prefix={<SearchOutlined />}
              allowClear
              onChange={(e) => setFilter((f) => ({ ...f, name: e.target.value || undefined, page: 1 }))}
              style={{ width: 200 }}
            />
            <Select
              placeholder="项目状态"
              allowClear
              style={{ width: 120 }}
              onChange={(v) => setFilter((f) => ({ ...f, status: v, page: 1 }))}
              options={[
                { label: '进行中', value: 'active' },
                { label: '暂停', value: 'paused' },
                { label: '已归档', value: 'archived' },
              ]}
            />
          </Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)}>
            创建项目
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
      </Card>

      <Modal
        title="创建项目"
        open={createModalOpen}
        onCancel={() => setCreateModalOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={createMutation.isPending}
      >
        <Form form={form} layout="vertical" onFinish={(v) => createMutation.mutate(v)}>
          <Form.Item name="key" label="项目标识" rules={[{ required: true }]}>
            <Input placeholder="如 MF, PROJ" />
          </Form.Item>
          <Form.Item name="name" label="项目名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="summary" label="项目简介">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="git_url" label="Git 地址">
            <Input />
          </Form.Item>
          <Form.Item name="cicd_url" label="CI/CD 地址">
            <Input />
          </Form.Item>
          <Form.Item name="doc_url" label="文档地址">
            <Input />
          </Form.Item>
          <Form.Item name="owner_id" label="负责人">
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ProjectList;
