import React, { useEffect, useRef, useState } from 'react';
import { Card, List, Input, Button, Badge, Tag, Space, Popconfirm, Empty, Spin, message } from 'antd';
import { MessageOutlined, DeleteOutlined, SendOutlined } from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { listComments, createComment, deleteComment, markCommentsRead } from '../../api/comment';
import { readerId } from '../../types/comment';
import Markdown from '../Markdown';

interface CommentPanelProps {
  /** Issue key (OZX-12) or UUID — the API accepts either. */
  issueKey: string;
  /**
   * Ids that were unread when this issue was opened. Kept by the page rather
   * than read from the query, because the panel marks the thread read on sight:
   * without a snapshot the "new" markers would vanish the moment they appeared.
   */
  newIds: Set<string>;
}

/**
 * CommentPanel is the issue's discussion thread: what someone wants the next
 * reader to know, kept apart from the automatic field history and from the
 * memories that outlive the issue.
 */
const CommentPanel: React.FC<CommentPanelProps> = ({ issueKey, newIds }) => {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState('');
  const me = readerId();

  const { data: thread, isLoading } = useQuery({
    queryKey: ['comments', issueKey],
    queryFn: () => listComments(issueKey),
    enabled: !!issueKey,
  });
  const comments = thread?.comments ?? [];

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['comments', issueKey] });

  // Opening the issue is reading it. Fire once per issue, after the thread has
  // loaded with something unread in it.
  const marked = useRef<string | null>(null);
  useEffect(() => {
    if (!thread || marked.current === issueKey) return;
    if (thread.summary.unread === 0) return;
    marked.current = issueKey;
    markCommentsRead(issueKey)
      .then(refresh)
      .catch(() => {
        // A failed receipt is not worth interrupting the reader — the comments
        // are on screen either way, and the next visit will retry.
        marked.current = null;
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [thread, issueKey]);

  const createMutation = useMutation({
    mutationFn: (body: string) => createComment(issueKey, { body }),
    onSuccess: () => {
      setDraft('');
      refresh();
    },
    onError: () => message.error('评论失败'),
  });

  const deleteMutation = useMutation({
    mutationFn: (commentId: string) => deleteComment(issueKey, commentId),
    onSuccess: () => {
      message.success('已删除');
      refresh();
    },
    onError: () => message.error('删除失败'),
  });

  const submit = () => {
    const body = draft.trim();
    if (!body) return;
    createMutation.mutate(body);
  };

  const title = (
    <Space>
      <MessageOutlined />
      <span>评论</span>
      <Badge count={newIds.size} title={`${newIds.size} 条未读`} />
    </Space>
  );

  return (
    <Card id="comments" title={title} style={{ marginTop: 16 }} extra={<span style={{ color: '#999' }}>以 {me} 的身份</span>}>
      {isLoading ? (
        <Spin />
      ) : comments.length === 0 ? (
        <Empty description="还没有评论" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <List
          dataSource={comments}
          renderItem={(c) => {
            const isNew = newIds.has(c.id);
            return (
              <List.Item
                key={c.id}
                style={{
                  alignItems: 'flex-start',
                  // A left rule is enough to pick the new ones out of the thread
                  // without recolouring the whole row.
                  borderLeft: isNew ? '3px solid #1890ff' : '3px solid transparent',
                  paddingLeft: 12,
                }}
                actions={[
                  <Popconfirm key="del" title="删除这条评论？" onConfirm={() => deleteMutation.mutate(c.id)}>
                    <Button type="text" size="small" danger icon={<DeleteOutlined />} />
                  </Popconfirm>,
                ]}
              >
                <div style={{ width: '100%' }}>
                  <Space size={8} style={{ marginBottom: 4 }}>
                    <strong>{c.author_id || '匿名'}</strong>
                    <span style={{ color: '#999', fontSize: 12 }}>
                      {dayjs(c.created_at).format('YYYY-MM-DD HH:mm')}
                    </span>
                    {isNew && <Tag color="blue">新</Tag>}
                  </Space>
                  <Markdown source={c.body} />
                </div>
              </List.Item>
            );
          }}
        />
      )}

      <div style={{ marginTop: 16 }}>
        <Input.TextArea
          rows={3}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="写下评论，支持 Markdown。Ctrl/⌘ + Enter 发送"
          onKeyDown={(e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
              e.preventDefault();
              submit();
            }
          }}
        />
        <Button
          type="primary"
          icon={<SendOutlined />}
          style={{ marginTop: 8 }}
          onClick={submit}
          disabled={!draft.trim()}
          loading={createMutation.isPending}
        >
          发表评论
        </Button>
      </div>
    </Card>
  );
};

export default CommentPanel;
