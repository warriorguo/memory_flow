import React, { useEffect, useState } from 'react';
import { Alert, Descriptions, Button, Card, List, Modal, Form, Input, Popconfirm, Select, Tag, Timeline, Space, Spin, message } from 'antd';
import { EditOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { getIssue, updateIssue, transitionIssueStatus, getIssueHistory, findSimilarIssues, markDuplicate } from '../../api/issue';
import { SimilarIssueList } from '../../components/SimilarIssues';
import { listAssets } from '../../api/asset';
import { listComments } from '../../api/comment';
import { listIssueMemories, markMemoriesRead, promoteMemory, setIssueEpic } from '../../api/epic';
import EpicPanel from '../../components/EpicPanel';
import EpicSelect from '../../components/EpicSelect';
import StatusTag from '../../components/StatusTag';
import PriorityBadge from '../../components/PriorityBadge';
import Markdown from '../../components/Markdown';
import AssetPanel from '../../components/AssetPanel';
import CommentPanel from '../../components/CommentPanel';
import DependencyPanel from '../../components/DependencyPanel';
import TagSelector from '../../components/TagSelector';
import { TagLabels } from '../../components/TagLabel';
import { ALLOWED_TRANSITIONS, CAN_MARK_DUPLICATE, ISSUE_STATUS_LABELS, ISSUE_TYPE_LABELS, PRIORITY_LABELS } from '../../types/common';
import type { IssueMemory } from '../../types/epic';
import type { IssueStatus } from '../../types/common';
import dayjs from 'dayjs';

const IssueDetail: React.FC = () => {
  const { issueKey } = useParams<{ issueKey: string }>();
  const id = issueKey;
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);
  const [dupOpen, setDupOpen] = useState(false);
  const [dupTarget, setDupTarget] = useState('');
  const [epicOpen, setEpicOpen] = useState(false);
  const [epicTarget, setEpicTarget] = useState<string | undefined>();
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

  // The issue's own memories plus, when it is in an epic, the epic's shared
  // ones with this reader's read state (MF-39).
  const { data: allMemories = [] } = useQuery({
    queryKey: ['issueMemories', id],
    queryFn: () => listIssueMemories(id!),
    enabled: !!id,
  });
  const ownMemories = allMemories.filter((m) => m.scope === 'issue');
  const epicMemories = allMemories.filter((m) => m.scope === 'epic');

  // Like comments: remember which shared memories were new or changed when the
  // page opened, then mark them read — the tags stay until the next visit.
  const [memSnapshot, setMemSnapshot] = useState<{ key?: string; marks: Record<string, 'new' | 'updated'> }>({ marks: {} });
  if (allMemories.length > 0 && memSnapshot.key !== id) {
    const marks: Record<string, 'new' | 'updated'> = {};
    for (const m of epicMemories) {
      if (m.updated) marks[m.id] = 'updated';
      else if (m.unread) marks[m.id] = 'new';
    }
    setMemSnapshot({ key: id, marks });
  }
  useEffect(() => {
    const changed = epicMemories.filter((m) => m.unread || m.updated);
    // A failed receipt only means the tags show again next time.
    if (changed.length) markMemoriesRead(changed).catch(() => undefined);
  }, [allMemories]); // eslint-disable-line react-hooks/exhaustive-deps

  const epicMutation = useMutation({
    mutationFn: (epic: string) => setIssueEpic(id!, epic),
    onSuccess: (res) => {
      const now = res.issue.epic_id ? epicTarget : undefined;
      message.success(now ? `已加入 ${now}` : `已移出 ${res.previous?.issue_key ?? 'Epic'}`);
      queryClient.invalidateQueries({ queryKey: ['issue', id] });
      queryClient.invalidateQueries({ queryKey: ['issueMemories', id] });
      queryClient.invalidateQueries({ queryKey: ['issueHistory', id] });
      queryClient.invalidateQueries({ queryKey: ['epic'] });
      queryClient.invalidateQueries({ queryKey: ['epics'] });
      setEpicOpen(false);
    },
    onError: (err: { response?: { data?: { error?: string } } }) => message.error(err.response?.data?.error ?? '操作失败'),
  });

  const promoteMutation = useMutation({
    mutationFn: ({ m, force }: { m: IssueMemory; force?: boolean }) => promoteMemory(m.id, false, force),
    onSuccess: (res) => {
      message.success(`已提升到 ${res.epic}，Epic 内的其他工作项都能看到`);
      queryClient.invalidateQueries({ queryKey: ['issueMemories'] });
    },
    onError: (err: { response?: { status?: number; data?: { error?: string; conflicts?: { title: string }[] } } }, vars) => {
      const conflicts = err.response?.data?.conflicts;
      if (err.response?.status === 409 && conflicts?.length) {
        Modal.confirm({
          title: 'Epic 上已有相似的 Memory',
          content: (
            <div>
              <p>这条结论可能和下面的 Epic Memory 是同一件事。同一决策请直接编辑那一条，避免出现两种说法。</p>
              <ul>{conflicts.map((c) => <li key={c.title}>{c.title}</li>)}</ul>
            </div>
          ),
          okText: '仍然提升',
          cancelText: '取消',
          onOk: () => promoteMutation.mutate({ m: vars.m, force: true }),
        });
        return;
      }
      message.error(err.response?.data?.error ?? '操作失败');
    },
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

  // Candidates for "mark as duplicate": what the duplicate detector thinks
  // this issue is the same as. Fetched only while the dialog is open.
  const { data: dupCandidates = [] } = useQuery({
    queryKey: ['similar-issues', 'of', issue?.issue_key],
    queryFn: () =>
      findSimilarIssues(issue!.project_id, {
        type: issue!.type,
        title: issue!.title,
        description: issue!.description,
        exclude_key: issue!.issue_key,
      }),
    enabled: dupOpen && !!issue,
  });

  const dupMutation = useMutation({
    mutationFn: (target: string) => markDuplicate(id!, target),
    onSuccess: (updated) => {
      message.success(`已标记为 ${updated.duplicate_of?.issue_key ?? dupTarget} 的重复`);
      queryClient.invalidateQueries({ queryKey: ['issue'] });
      queryClient.invalidateQueries({ queryKey: ['issueHistory', id] });
      queryClient.invalidateQueries({ queryKey: ['issues'] });
      queryClient.invalidateQueries({ queryKey: ['progress-summary'] });
      setDupOpen(false);
      setDupTarget('');
    },
    onError: (err: { response?: { data?: { error?: string } } }) => message.error(err.response?.data?.error ?? '操作失败'),
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

      <Card
        title={<span>{issue.issue_key} - {issue.title}<TagLabels tags={issue.tags} /></span>}
        extra={
          <Button
            icon={<EditOutlined />}
            onClick={() => {
              form.setFieldsValue({ ...issue, tag_ids: (issue.tags ?? []).map((t) => t.id) });
              setEditOpen(true);
            }}
          >
            编辑
          </Button>
        }
      >
        <Descriptions column={2} bordered>
          <Descriptions.Item label="类型">{ISSUE_TYPE_LABELS[issue.type] ?? issue.type}</Descriptions.Item>
          <Descriptions.Item label="优先级"><PriorityBadge priority={issue.priority} /></Descriptions.Item>
          <Descriptions.Item label="状态">
            <StatusTag status={issue.status} />
            {issue.duplicate_of && (
              <span>
                重复于 <a onClick={() => navigate(`/issues/${issue.duplicate_of!.issue_key}`)}>{issue.duplicate_of.issue_key}</a>
                {' '}{issue.duplicate_of.title}
              </span>
            )}
          </Descriptions.Item>
          {issue.type !== 'epic' && (
            <Descriptions.Item label="所属 Epic" span={2}>
              <Space>
                {issue.epic ? (
                  <span>
                    <a onClick={() => navigate(`/issues/${issue.epic!.issue_key}`)}>{issue.epic.issue_key}</a> {issue.epic.title}
                  </span>
                ) : (
                  '-'
                )}
                <Button size="small" type="link" onClick={() => { setEpicTarget(issue.epic?.issue_key); setEpicOpen(true); }}>
                  {issue.epic ? '更改' : '加入 Epic'}
                </Button>
              </Space>
            </Descriptions.Item>
          )}
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
          {issue.duplicates && issue.duplicates.length > 0 && (
            <Descriptions.Item label="已并入的重复" span={2}>
              <Space direction="vertical" size={2}>
                {issue.duplicates.map((d) => (
                  <span key={d.id}>
                    <a onClick={() => navigate(`/issues/${d.issue_key}`)}>{d.issue_key}</a> {d.title}
                  </span>
                ))}
              </Space>
            </Descriptions.Item>
          )}
          <Descriptions.Item label="标签" span={2}>
            {issue.tags && issue.tags.length > 0 ? <TagLabels tags={issue.tags} /> : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="创建时间">{dayjs(issue.created_at).format('YYYY-MM-DD HH:mm')}</Descriptions.Item>
          <Descriptions.Item label="更新时间">{dayjs(issue.updated_at).format('YYYY-MM-DD HH:mm')}</Descriptions.Item>
        </Descriptions>

        {(allowedNext.length > 0 || CAN_MARK_DUPLICATE.includes(issue.status)) && (
          <div style={{ marginTop: 16 }}>
            <span style={{ marginRight: 8 }}>状态流转:</span>
            <Space wrap>
              {allowedNext.map((s) => (
                <Button key={s} size="small" onClick={() => transitionMutation.mutate(s)} loading={transitionMutation.isPending}>
                  {ISSUE_STATUS_LABELS[s]}
                </Button>
              ))}
              {CAN_MARK_DUPLICATE.includes(issue.status) && (
                <Button size="small" onClick={() => setDupOpen(true)}>
                  {issue.status === 'duplicate' ? '改为其他单的重复' : '标记为重复'}
                </Button>
              )}
            </Space>
          </div>
        )}
      </Card>

      <AssetPanel issueKey={id!} />

      <DependencyPanel issueKey={id!} issueId={issue.id} />

      {issue.type === 'epic' && <EpicPanel epicKey={issue.issue_key} />}

      {epicMemories.length > 0 && (
        <Card
          title={`Epic 共享 Memory（来自 ${epicMemories[0].epic_key}）`}
          extra={<span style={{ color: '#999' }}>Epic 内所有工作项都能看到</span>}
          style={{ marginTop: 16 }}
        >
          <List
            dataSource={epicMemories}
            renderItem={(m) => (
              <List.Item key={m.id}>
                <List.Item.Meta
                  title={
                    <Space>
                      <Tag color={m.type === 'recall' ? 'blue' : 'green'}>{m.type}</Tag>
                      <span>{m.title}</span>
                      {memSnapshot.marks[m.id] === 'new' && <Tag color="red">新</Tag>}
                      {memSnapshot.marks[m.id] === 'updated' && <Tag color="orange">上次读后有更新</Tag>}
                    </Space>
                  }
                  description={<Markdown source={m.content} />}
                />
              </List.Item>
            )}
          />
        </Card>
      )}

      {ownMemories.length > 0 && (
        <Card title={issue.type === 'epic' ? 'Epic 共享 Memory' : '关联 Memory'} style={{ marginTop: 16 }}>
          <List
            dataSource={ownMemories}
            renderItem={(m) => (
              <List.Item
                key={m.id}
                actions={
                  issue.epic
                    ? [
                        <Popconfirm
                          key="promote"
                          title={`把这条 Memory 移到 ${issue.epic.issue_key}，让 Epic 内其他工作项也能看到？`}
                          onConfirm={() => promoteMutation.mutate({ m })}
                        >
                          <a>提升到 Epic</a>
                        </Popconfirm>,
                      ]
                    : undefined
                }
              >
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
        title={`将 ${issue.issue_key} 标记为重复`}
        open={dupOpen}
        onCancel={() => setDupOpen(false)}
        onOk={() => dupTarget.trim() && dupMutation.mutate(dupTarget.trim())}
        okButtonProps={{ disabled: !dupTarget.trim() }}
        confirmLoading={dupMutation.isPending}
        width={640}
      >
        <p>本单会关闭为「重复」并指向目标单；本单的证据保留，目标单会列出它并收到一条评论。</p>
        <Input
          placeholder="目标工作项编号，如 MF-12"
          value={dupTarget}
          onChange={(e) => setDupTarget(e.target.value)}
          onPressEnter={() => dupTarget.trim() && dupMutation.mutate(dupTarget.trim())}
        />
        {dupCandidates.length > 0 && (
          <div style={{ marginTop: 12 }}>
            <div style={{ color: '#999' }}>疑似重复：</div>
            <SimilarIssueList items={dupCandidates} onPick={(c) => setDupTarget(c.key)} />
          </div>
        )}
      </Modal>

      <Modal
        title={`${issue.issue_key} 所属 Epic`}
        open={epicOpen}
        onCancel={() => setEpicOpen(false)}
        footer={[
          issue.epic && (
            <Button key="rm" danger onClick={() => epicMutation.mutate('')} loading={epicMutation.isPending}>
              移出 Epic
            </Button>
          ),
          <Button key="cancel" onClick={() => setEpicOpen(false)}>取消</Button>,
          <Button key="ok" type="primary" disabled={!epicTarget} loading={epicMutation.isPending}
            onClick={() => epicTarget && epicMutation.mutate(epicTarget)}>
            确定
          </Button>,
        ]}
      >
        <p>一张工作项同一时间只属于一个 Epic；选择其他 Epic 会直接改挂。Epic 可以来自其他项目，输入编号即可。</p>
        <Space direction="vertical" style={{ width: '100%' }}>
          <EpicSelect projectId={issue.project_id} value={epicTarget} onChange={(v) => setEpicTarget(v)} style={{ width: '100%' }} />
          <Input
            placeholder="或输入其他项目的 Epic 编号，如 OZX-1642"
            onChange={(e) => setEpicTarget(e.target.value.trim().toUpperCase() || undefined)}
          />
        </Space>
      </Modal>

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
          <Form.Item name="tag_ids" label="标签">
            <TagSelector allowCreate />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default IssueDetail;
