import React, { useState, useMemo } from 'react';
import { Spin, message, Badge, Select, Input, Space, Typography, Button, Modal, Form } from 'antd';
import { PlusOutlined, FilterOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { DndContext, useDroppable, useDraggable, useSensor, useSensors, PointerSensor, DragOverlay } from '@dnd-kit/core';
import type { DragEndEvent, DragStartEvent } from '@dnd-kit/core';
import { listIssues, transitionIssueStatus, createIssue } from '../../api/issue';
import { useNavigate, useParams } from 'react-router-dom';
import IssueCard from '../../components/IssueCard';
import { ISSUE_STATUS_LABELS, ALLOWED_TRANSITIONS, PRIORITY_COLORS } from '../../types/common';
import type { Issue, CreateIssueRequest } from '../../types/issue';
import type { IssueStatus, IssueType, Priority } from '../../types/common';

const { Text } = Typography;

const KANBAN_STATUSES: IssueStatus[] = ['todo', 'in_progress', 'review', 'testing', 'done'];

const COLUMN_COLORS: Record<IssueStatus, { bg: string; header: string; border: string }> = {
  todo: { bg: '#fafafa', header: '#d9d9d9', border: '#d9d9d9' },
  in_progress: { bg: '#e6f7ff', header: '#1890ff', border: '#91d5ff' },
  review: { bg: '#fffbe6', header: '#faad14', border: '#ffe58f' },
  testing: { bg: '#f9f0ff', header: '#722ed1', border: '#d3adf7' },
  done: { bg: '#f6ffed', header: '#52c41a', border: '#b7eb8f' },
  closed: { bg: '#fafafa', header: '#8c8c8c', border: '#d9d9d9' },
  suspended: { bg: '#fffbe6', header: '#faad14', border: '#ffe58f' },
  rejected: { bg: '#fff2f0', header: '#ff4d4f', border: '#ffa39e' },
};

interface Props {
  projectId?: string;
}

function DroppableColumn({ status, count, children }: { status: IssueStatus; count: number; children: React.ReactNode }) {
  const { setNodeRef, isOver } = useDroppable({ id: status });
  const colors = COLUMN_COLORS[status];
  return (
    <div
      ref={setNodeRef}
      style={{
        flex: 1,
        minWidth: 260,
        maxWidth: 360,
        background: isOver ? '#f0f5ff' : colors.bg,
        borderRadius: 8,
        display: 'flex',
        flexDirection: 'column',
        border: isOver ? '2px dashed #1890ff' : `1px solid ${colors.border}`,
        transition: 'all 0.2s',
      }}
    >
      <div style={{
        padding: '10px 14px',
        borderBottom: `2px solid ${colors.header}`,
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
      }}>
        <Text strong>{ISSUE_STATUS_LABELS[status]}</Text>
        <Badge count={count} style={{ backgroundColor: colors.header }} />
      </div>
      <div style={{ padding: 10, flex: 1, overflowY: 'auto', minHeight: 300 }}>
        {children}
      </div>
    </div>
  );
}

function DraggableCard({ issue, onClick }: { issue: Issue; onClick: () => void }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({ id: issue.id, data: { issue } });
  const style: React.CSSProperties = {
    ...(transform ? { transform: `translate3d(${transform.x}px, ${transform.y}px, 0)` } : {}),
    opacity: isDragging ? 0.4 : 1,
    cursor: 'grab',
  };

  return (
    <div ref={setNodeRef} style={style} {...listeners} {...attributes}>
      <IssueCard issue={issue} onClick={onClick} />
    </div>
  );
}

const KanbanBoard: React.FC<Props> = ({ projectId: propProjectId }) => {
  const params = useParams<{ projectId: string }>();
  const projectId = propProjectId || params.projectId!;
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [activeIssue, setActiveIssue] = useState<Issue | null>(null);
  const [filterType, setFilterType] = useState<IssueType | undefined>();
  const [filterPriority, setFilterPriority] = useState<Priority | undefined>();
  const [filterKeyword, setFilterKeyword] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [form] = Form.useForm();

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } })
  );

  const { data, isLoading } = useQuery({
    queryKey: ['issues', projectId, { page_size: 500 }],
    queryFn: () => listIssues(projectId, { page: 1, page_size: 500 }),
    enabled: !!projectId,
  });

  const transitionMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) => transitionIssueStatus(id, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['issues', projectId] });
    },
    onError: () => {
      message.error('状态流转失败');
    },
  });

  const createMutation = useMutation({
    mutationFn: (req: CreateIssueRequest) => createIssue(projectId, req),
    onSuccess: () => {
      message.success('创建成功');
      queryClient.invalidateQueries({ queryKey: ['issues', projectId] });
      setCreateOpen(false);
      form.resetFields();
    },
    onError: () => message.error('创建失败'),
  });

  const filteredIssues = useMemo(() => {
    let items = data?.data || [];
    if (filterType) items = items.filter(i => i.type === filterType);
    if (filterPriority) items = items.filter(i => i.priority === filterPriority);
    if (filterKeyword) {
      const kw = filterKeyword.toLowerCase();
      items = items.filter(i => i.title.toLowerCase().includes(kw) || i.issue_key.toLowerCase().includes(kw));
    }
    return items;
  }, [data, filterType, filterPriority, filterKeyword]);

  const columnIssues = useMemo(() => {
    const map: Record<IssueStatus, Issue[]> = {} as any;
    for (const s of KANBAN_STATUSES) map[s] = [];
    for (const issue of filteredIssues) {
      if (map[issue.status]) map[issue.status].push(issue);
    }
    // Sort by priority within each column
    for (const s of KANBAN_STATUSES) {
      map[s].sort((a, b) => a.priority.localeCompare(b.priority));
    }
    return map;
  }, [filteredIssues]);

  const handleDragStart = (event: DragStartEvent) => {
    setActiveIssue(event.active.data.current?.issue as Issue);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    setActiveIssue(null);
    const { active, over } = event;
    if (!over) return;
    const issue = active.data.current?.issue as Issue;
    const newStatus = over.id as string;
    if (issue.status === newStatus) return;

    const allowed = ALLOWED_TRANSITIONS[issue.status] || [];
    if (!allowed.includes(newStatus as IssueStatus)) {
      message.warning(`不允许从「${ISSUE_STATUS_LABELS[issue.status]}」流转到「${ISSUE_STATUS_LABELS[newStatus as IssueStatus]}」`);
      return;
    }
    transitionMutation.mutate({ id: issue.id, status: newStatus });
  };

  if (isLoading) return <Spin style={{ display: 'block', margin: '100px auto' }} />;

  const totalVisible = filteredIssues.length;

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space>
          <FilterOutlined style={{ color: '#999' }} />
          <Select
            placeholder="类型"
            allowClear
            style={{ width: 120 }}
            value={filterType}
            onChange={setFilterType}
            options={[
              { label: '需求', value: 'requirement' },
              { label: 'Bug', value: 'bug' },
            ]}
          />
          <Select
            placeholder="优先级"
            allowClear
            style={{ width: 120 }}
            value={filterPriority}
            onChange={setFilterPriority}
            options={[
              { label: 'P0 - 紧急', value: 'P0' },
              { label: 'P1 - 重要', value: 'P1' },
              { label: 'P2 - 一般', value: 'P2' },
            ]}
          />
          <Input.Search
            placeholder="搜索标题或编号"
            allowClear
            style={{ width: 200 }}
            value={filterKeyword}
            onChange={e => setFilterKeyword(e.target.value)}
          />
          <Text type="secondary" style={{ fontSize: 13 }}>
            共 {totalVisible} 项
          </Text>
        </Space>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          新建
        </Button>
      </div>

      <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
        <div style={{ display: 'flex', gap: 12, overflowX: 'auto', paddingBottom: 8 }}>
          {KANBAN_STATUSES.map((status) => (
            <DroppableColumn key={status} status={status} count={columnIssues[status].length}>
              {columnIssues[status].map((issue) => (
                <DraggableCard key={issue.id} issue={issue} onClick={() => navigate(`/issues/${issue.id}`)} />
              ))}
            </DroppableColumn>
          ))}
        </div>
        <DragOverlay>
          {activeIssue ? <IssueCard issue={activeIssue} /> : null}
        </DragOverlay>
      </DndContext>

      <Modal
        title="新建工作项"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={createMutation.isPending}
      >
        <Form form={form} layout="vertical" onFinish={(v) => createMutation.mutate(v)}>
          <Form.Item name="type" label="类型" rules={[{ required: true }]} initialValue="requirement">
            <Select options={[
              { label: '需求', value: 'requirement' },
              { label: 'Bug', value: 'bug' },
            ]} />
          </Form.Item>
          <Form.Item name="title" label="标题" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="priority" label="优先级" initialValue="P2">
            <Select options={[
              { label: 'P0 - 紧急', value: 'P0' },
              { label: 'P1 - 重要', value: 'P1' },
              { label: 'P2 - 一般', value: 'P2' },
            ]} />
          </Form.Item>
          <Form.Item name="assignee_id" label="负责人">
            <Input />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default KanbanBoard;
