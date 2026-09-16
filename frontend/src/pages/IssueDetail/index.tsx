import React, { useState } from 'react';
import { Alert, Descriptions, Button, Card, List, Modal, Form, Input, Select, Tag, Timeline, Space, Spin, message } from 'antd';
import { EditOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { getIssue, updateIssue, transitionIssueStatus, getIssueHistory } from '../../api/issue';
import { listAssets } from '../../api/asset';
import { listComments } from '../../api/comment';
import { listMemories } from '../../api/memory';
import StatusTag from '../../components/StatusTag';
import PriorityBadge from '../../components/PriorityBadge';
import Markdown from '../../components/Markdown';
import AssetPanel from '../../components/AssetPanel';
import CommentPanel from '../../components/CommentPanel';
import { ALLOWED_TRANSITIONS, ISSUE_STATUS_LABELS, PRIORITY_LABELS } from '../../types/common';
import type { IssueStatus } from '../../types/common';
import dayjs from 'dayjs';

const IssueDetail: React.FC = () => {
  const { issueKey } = useParams<{ issueKey: string }>();
  const id = issueKey;
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);
  const [form] = Form.useForm();

  const { data: issue, isLoading } = useQuery({
    queryKey: ['issue', id],
    queryFn: () => getIssue(id!),
    enabled: !!id,
  });

  // Shares AssetPanel's query key, so the description and the panel read the
  // same list and one upload refreshes both.
  const { data: assets = [] } = useQuery({
    queryKey: ['assets', id],
    queryFn: () => listAssets(id!),
    enabled: !!id,
  });

  const { data: history = [] } = useQuery({
    queryKey: ['issueHistory', id],
    queryFn: () => getIssueHistory(id!),
    enabled: !!id,
  });

  // Shares CommentPanel's query key: the banner and the thread read one list.
  const { data: thread } = useQuery({
    queryKey: ['comments', id],
    queryFn: () => listComments(id!),
    enabled: !!id,
  });

  // Memories point at their source object, so the issue's UUID is the key —
  // not the issue key in the URL.
  const { data: memories } = useQuery({
    queryKey: ['issueMemories', issue?.id],
    queryFn: () => listMemories({ source_object_id: issue!.id, page_size: 50 }),
    enabled: !!issue?.id,
  });

  // The comments that were unread when this issue was opened. CommentPanel marks
  // the thread read on sight, so this has to remember what was new rather than
  // recompute it — otherwise the banner would disappear before it was read. The
  // snapshot is taken once per issue, on the thread's first load.
  const [snapshot, setSnapshot] = useState<{ key?: string; ids: Set<string> }>({ ids: new Set() });
  if (thread && snapshot.key !== id) {
    setSnapshot({ key: id, ids: new Set(thread.comments.filter((c) => c.unread).map((c) => c.id)) });
  }
  const newCommentIds = snapshot.ids;

  const updateMutation = useMutation({
    mutationFn: (values: any) => updateIssue(id!, values),
    onSuccess: () => {
      message.success('更新成功');
      queryClient.invalidateQueries({ queryKey: ['issue', id] });
      queryClient.invalidateQueries({ queryKey: ['issues'] });
      setEditOpen(false);
    },
    onError: () => message.error('操作失败'),
  });

  const transitionMutation = useMutation({
    mutationFn: (status: string) => transitionIssueStatus(id!, status),
    onSuccess: () => {
      message.success('状态更新成功');
      queryClient.invalidateQueries({ queryKey: ['issue', id] });
      queryClient.invalidateQueries({ queryKey: ['issueHistory', id] });
      queryClient.invalidateQueries({ queryKey: ['issues'] });
      queryClient.invalidateQueries({ queryKey: ['progress-summary'] });
    },
    onError: () => message.error('操作失败'),
  });

  if (isLoading) return <Spin />;
  if (!issue) return null;

  const allowedNext = ALLOWED_TRANSITIONS[issue.status as IssueStatus] || [];

  return (
    <div>
      <Button type="link" onClick={() => navigate(-1)} style={{ padding: 0, marginBottom: 16 }}>
        &larr; 返回
      </Button>

      {newCommentIds.size > 0 && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={`有 ${newCommentIds.size} 条新评论`}
          description="其他人在这个工作项上留了言，建议先读一读再动手。"
          action={
            <Button
              size="small"
              onClick={() => document.getElementById('comments')?.scrollIntoView({ behavior: 'smooth' })}
            >
              查看评论
            </Button>
          }
        />
      )}

      <Card title={<span>{issue.issue_key} - {issue.title}</span>} extra={<Button icon={<EditOutlined />} onClick={() => { form.setFieldsValue(issue); setEditOpen(true); }}>编辑</Button>}>
        <Descriptions column={2} bordered>
          <Descriptions.Item label="类型">{issue.type === 'requirement' ? '需求' : '缺陷'}</Descriptions.Item>
          <Descriptions.Item label="优先级"><PriorityBadge priority={issue.priority} /></Descriptions.Item>
          <Descriptions.Item label="状态"><StatusTag status={issue.status} /></Descriptions.Item>
          <Descriptions.Item label="指派人">{issue.assignee_id || '-'}</Descriptions.Item>
          <Descriptions.Item label="创建人">{issue.creator_id || '-'}</Descriptions.Item>
          <Descriptions.Item label="来源">{issue.source || '-'}</Descriptions.Item>
          <Descriptions.Item label="关联版本">{issue.version || '-'}</Descriptions.Item>
          <Descriptions.Item label="Git">{issue.git_url ? <a href={issue.git_url} target="_blank" rel="noreferrer">{issue.git_url}</a> : '-'}</Descriptions.Item>
          <Descriptions.Item label="PR">{issue.pr_url ? <a href={issue.pr_url} target="_blank" rel="noreferrer">{issue.pr_url}</a> : '-'}</Descriptions.Item>
          <Descriptions.Item label="文档">{issue.doc_url ? <a href={issue.doc_url} target="_blank" rel="noreferrer">{issue.doc_url}</a> : '-'}</Descriptions.Item>
          <Descriptions.Item label="描述" span={2}>
            {/* assetContext lets asset:<文件名> in the description resolve to
                this issue's attachments and render inline. */}
            <Markdown source={issue.description} assetContext={{ issueKey: id!, assets }} />
          </Descriptions.Item>
          <Descriptions.Item label="标签" span={2}>
            {issue.tags && issue.tags.length > 0 ? issue.tags.map((t) => <Tag key={t.id} color={t.color}>{t.name}</Tag>) : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="创建时间">{dayjs(issue.created_at).format('YYYY-MM-DD HH:mm')}</Descriptions.Item>
          <Descriptions.Item label="更新时间">{dayjs(issue.updated_at).format('YYYY-MM-DD HH:mm')}</Descriptions.Item>
        </Descriptions>

        {allowedNext.length > 0 && (
          <div style={{ marginTop: 16 }}>
            <span style={{ marginRight: 8 }}>状态流转:</span>
            <Space>
              {allowedNext.map((s) => (
                <Button key={s} size="small" onClick={() => transitionMutation.mutate(s)} loading={transitionMutation.isPending}>
                  {ISSUE_STATUS_LABELS[s]}
                </Button>
              ))}
            </Space>
          </div>
        )}
      </Card>

      <AssetPanel issueKey={id!} />

      {memories && memories.data.length > 0 && (
        <Card title="关联 Memory" style={{ marginTop: 16 }}>
          <List
            dataSource={memories.data}
            renderItem={(m) => (
              <List.Item key={m.id}>
                <List.Item.Meta
                  title={
                    <Space>
                      <Tag color={m.type === 'recall' ? 'blue' : 'green'}>{m.type}</Tag>
                      <span>{m.title}</span>
                    </Space>
                  }
                  description={<Markdown source={m.content} />}
                />
              </List.Item>
            )}
          />
        </Card>
      )}

      <CommentPanel issueKey={id!} newIds={newCommentIds} />

      <Card title="操作历史" style={{ marginTop: 16 }}>
        <Timeline
          items={history.map((h) => ({
            children: (
              <div>
                <strong>{h.field_name}</strong>: {h.old_value || '(空)'} &rarr; {h.new_value || '(空)'}
                <div style={{ color: '#999', fontSize: 12 }}>
                  {h.operator_id} | {dayjs(h.created_at).format('YYYY-MM-DD HH:mm:ss')}
                </div>
              </div>
            ),
          }))}
        />
      </Card>

      <Modal
        title="编辑工作项"
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={updateMutation.isPending}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={(v) => updateMutation.mutate(v)}>
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
          <Form.Item name="git_url" label="Git 地址">
            <Input />
          </Form.Item>
          <Form.Item name="pr_url" label="PR 地址">
            <Input />
          </Form.Item>
          <Form.Item name="doc_url" label="文档地址">
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default IssueDetail;
