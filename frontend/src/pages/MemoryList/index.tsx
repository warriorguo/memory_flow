import React, { useState, useEffect } from 'react';
import { Table, Button, Input, Select, Space, Card, Modal, Form, Tag, message } from 'antd';
import { PlusOutlined, SearchOutlined, EyeOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listMemories, createMemory, updateMemory, deleteMemory } from '../../api/memory';
import TagSelector from '../../components/TagSelector';
import type { MemoryFilter, CreateMemoryRequest, Memory, UpdateMemoryRequest } from '../../types/memory';
import type { MemoryType } from '../../types/common';
import dayjs from 'dayjs';

interface Props {
  projectId?: string;
}

const MemoryListView: React.FC<Props> = ({ projectId }) => {
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState<MemoryFilter>({ project_id: projectId, page: 1, page_size: 20 });

  useEffect(() => {
    setFilter(f => ({ ...f, project_id: projectId, page: 1 }));
  }, [projectId]);

  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [selectedMemory, setSelectedMemory] = useState<Memory | null>(null);
  const [form] = Form.useForm();
  const [editForm] = Form.useForm();

  const { data, isLoading } = useQuery({
    queryKey: ['memories', filter],
    queryFn: () => listMemories(filter),
  });

  const createMutation = useMutation({
    mutationFn: (req: CreateMemoryRequest) => createMemory(req),
    onSuccess: () => {
      message.success('Memory 创建成功');
      queryClient.invalidateQueries({ queryKey: ['memories'] });
      setCreateOpen(false);
      form.resetFields();
    },
    onError: () => message.error('操作失败'),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, req }: { id: string; req: UpdateMemoryRequest }) => updateMemory(id, req),
    onSuccess: () => {
      message.success('更新成功');
      queryClient.invalidateQueries({ queryKey: ['memories'] });
      setEditOpen(false);
    },
    onError: () => message.error('操作失败'),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteMemory(id),
    onSuccess: () => {
      message.success('已删除');
      queryClient.invalidateQueries({ queryKey: ['memories'] });
    },
    onError: () => message.error('操作失败'),
  });

  const columns = [
    {
      title: '标题',
      dataIndex: 'title',
      render: (title: string, record: Memory) => (
        <a onClick={() => { setSelectedMemory(record); setDetailOpen(true); }}>{title}</a>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 80,
      render: (t: string) => <Tag color={t === 'recall' ? 'blue' : 'green'}>{t === 'recall' ? 'Recall' : 'Write'}</Tag>,
    },
    { title: '来源类型', dataIndex: 'source_object_type', width: 100 },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      width: 160,
      render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '操作',
      width: 150,
      render: (_: any, record: Memory) => (
        <Space>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => { setSelectedMemory(record); setDetailOpen(true); }} />
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => { setSelectedMemory(record); editForm.setFieldsValue(record); setEditOpen(true); }} />
          <Button
            type="link"
            size="small"
            danger
            icon={<DeleteOutlined />}
            onClick={() => Modal.confirm({ title: '确认删除？', onOk: () => deleteMutation.mutate(record.id) })}
          />
        </Space>
      ),
    },
  ];

  return (
    <Card>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Space>
          <Input
            placeholder="搜索关键字"
            prefix={<SearchOutlined />}
            allowClear
            onChange={(e) => setFilter((f) => ({ ...f, keyword: e.target.value || undefined, page: 1 }))}
            style={{ width: 200 }}
          />
          <Select
            placeholder="类型"
            allowClear
            style={{ width: 120 }}
            onChange={(v) => setFilter((f) => ({ ...f, type: v as MemoryType, page: 1 }))}
            options={[
              { label: 'Recall', value: 'recall' },
              { label: 'Write', value: 'write' },
            ]}
          />
        </Space>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          创建 Memory
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

      <Modal title="创建 Memory" open={createOpen} onCancel={() => setCreateOpen(false)} onOk={() => form.submit()} confirmLoading={createMutation.isPending} width={600}>
        <Form form={form} layout="vertical" onFinish={(v) => createMutation.mutate({ ...v, project_id: projectId || v.project_id })}>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select options={[{ label: 'Recall', value: 'recall' }, { label: 'Write', value: 'write' }]} />
          </Form.Item>
          <Form.Item name="title" label="标题" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="content" label="内容" rules={[{ required: true }]}>
            <Input.TextArea rows={6} />
          </Form.Item>
          <Form.Item name="source_object_type" label="来源对象类型">
            <Select allowClear options={[{ label: 'Project', value: 'project' }, { label: 'Requirement', value: 'requirement' }, { label: 'Bug', value: 'bug' }]} />
          </Form.Item>
          <Form.Item name="source_object_id" label="来源对象 ID">
            <Input />
          </Form.Item>
          <Form.Item name="tag_ids" label="标签">
            <TagSelector />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="编辑 Memory" open={editOpen} onCancel={() => setEditOpen(false)} onOk={() => editForm.submit()} confirmLoading={updateMutation.isPending} width={600}>
        <Form form={editForm} layout="vertical" onFinish={(v) => updateMutation.mutate({ id: selectedMemory!.id, req: v })}>
          <Form.Item name="title" label="标题" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="content" label="内容" rules={[{ required: true }]}>
            <Input.TextArea rows={6} />
          </Form.Item>
          <Form.Item name="type" label="类型">
            <Select options={[{ label: 'Recall', value: 'recall' }, { label: 'Write', value: 'write' }]} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="Memory 详情" open={detailOpen} onCancel={() => setDetailOpen(false)} footer={null} width={600}>
        {selectedMemory && (
          <div>
            <h3>{selectedMemory.title}</h3>
            <p><Tag color={selectedMemory.type === 'recall' ? 'blue' : 'green'}>{selectedMemory.type}</Tag></p>
            <div style={{ whiteSpace: 'pre-wrap', background: '#fafafa', padding: 16, borderRadius: 8, marginBottom: 16 }}>
              {selectedMemory.content}
            </div>
            <p style={{ color: '#999' }}>
              创建于 {dayjs(selectedMemory.created_at).format('YYYY-MM-DD HH:mm')} |
              更新于 {dayjs(selectedMemory.updated_at).format('YYYY-MM-DD HH:mm')}
            </p>
          </div>
        )}
      </Modal>
    </Card>
  );
};

export default MemoryListView;
