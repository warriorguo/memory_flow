import React, { useState, useEffect, useCallback } from 'react';
import { Tabs, Descriptions, Button, Card, Modal, Form, Input, Select, Spin, message } from 'antd';
import { EditOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getProject, updateProject } from '../../api/project';
import { useParams, useLocation, useNavigate } from 'react-router-dom';
import IssueListView from '../IssueList';
import KanbanBoard from '../KanbanBoard';
import ProgressDashboard from '../ProgressDashboard';
import MemoryListView from '../MemoryList';
import dayjs from 'dayjs';

const TAB_KEYS = ['overview', 'issues', 'kanban', 'progress', 'memory'] as const;
type TabKey = typeof TAB_KEYS[number];

function getTabFromHash(hash: string): TabKey {
  const key = hash.replace('#', '') as TabKey;
  return TAB_KEYS.includes(key) ? key : 'overview';
}

const ProjectDetail: React.FC = () => {
  const { projectKey } = useParams<{ projectKey: string }>();
  const projectId = projectKey;
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<TabKey>(() => getTabFromHash(location.hash));
  const [editOpen, setEditOpen] = useState(false);
  const [form] = Form.useForm();

  const handleTabChange = useCallback((key: string) => {
    setActiveTab(key as TabKey);
    navigate({ hash: key === 'overview' ? '' : key }, { replace: false });
  }, [navigate]);

  useEffect(() => {
    const onHashChange = () => setActiveTab(getTabFromHash(window.location.hash));
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  }, []);

  const { data: project, isLoading } = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => getProject(projectId!),
    enabled: !!projectId,
  });

  const updateMutation = useMutation({
    mutationFn: (values: any) => updateProject(projectId!, values),
    onSuccess: () => {
      message.success('更新成功');
      queryClient.invalidateQueries({ queryKey: ['project', projectId] });
      queryClient.invalidateQueries({ queryKey: ['projects'] });
      setEditOpen(false);
    },
    onError: () => message.error('操作失败'),
  });

  if (isLoading) return <Spin />;
  if (!project) return null;

  const handleEdit = () => {
    form.setFieldsValue(project);
    setEditOpen(true);
  };

  const tabItems = [
    {
      key: 'overview',
      label: '概览',
      children: (
        <Card extra={<Button icon={<EditOutlined />} onClick={handleEdit}>编辑</Button>}>
          <Descriptions column={2} bordered>
            <Descriptions.Item label="项目标识">{project.key}</Descriptions.Item>
            <Descriptions.Item label="项目名称">{project.name}</Descriptions.Item>
            <Descriptions.Item label="负责人">{project.owner_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="状态">{project.status}</Descriptions.Item>
            <Descriptions.Item label="Git 地址" span={2}>
              {project.git_url ? <a href={project.git_url} target="_blank" rel="noreferrer">{project.git_url}</a> : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="CI/CD 地址" span={2}>
              {project.cicd_url ? <a href={project.cicd_url} target="_blank" rel="noreferrer">{project.cicd_url}</a> : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="文档地址" span={2}>
              {project.doc_url ? <a href={project.doc_url} target="_blank" rel="noreferrer">{project.doc_url}</a> : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="项目简介" span={2}>{project.summary || '-'}</Descriptions.Item>
            <Descriptions.Item label="项目描述" span={2}>
              <div style={{ whiteSpace: 'pre-wrap' }}>{project.description || '-'}</div>
            </Descriptions.Item>
            <Descriptions.Item label="设计原则" span={2}>
              <div style={{ whiteSpace: 'pre-wrap' }}>{project.design_principles || '-'}</div>
            </Descriptions.Item>
            <Descriptions.Item label="创建时间">{dayjs(project.created_at).format('YYYY-MM-DD HH:mm')}</Descriptions.Item>
            <Descriptions.Item label="更新时间">{dayjs(project.updated_at).format('YYYY-MM-DD HH:mm')}</Descriptions.Item>
          </Descriptions>
        </Card>
      ),
    },
    {
      key: 'issues',
      label: '需求 / Bug',
      children: <IssueListView projectId={projectId!} />,
    },
    {
      key: 'kanban',
      label: '看板',
      children: <KanbanBoard projectId={projectId!} />,
    },
    {
      key: 'progress',
      label: '进度',
      children: <ProgressDashboard projectId={projectId!} />,
    },
    {
      key: 'memory',
      label: 'Memory',
      children: <MemoryListView projectId={projectId} />,
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>{project.name}</h2>
      <Tabs items={tabItems} activeKey={activeTab} onChange={handleTabChange} />

      <Modal
        title="编辑项目"
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={updateMutation.isPending}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={(v) => updateMutation.mutate(v)}>
          <Form.Item name="name" label="项目名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="summary" label="项目简介">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="description" label="项目描述">
            <Input.TextArea rows={4} />
          </Form.Item>
          <Form.Item name="design_principles" label="设计原则">
            <Input.TextArea rows={3} />
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
          <Form.Item name="status" label="状态">
            <Select options={[
              { label: '进行中', value: 'active' },
              { label: '暂停', value: 'paused' },
              { label: '已归档', value: 'archived' },
            ]} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ProjectDetail;
