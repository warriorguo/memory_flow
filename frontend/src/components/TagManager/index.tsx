import React, { useState } from 'react';
import { Modal, Table, Input, ColorPicker, Button, Space, message } from 'antd';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listTags, updateTag } from '../../api/tag';
import TagLabel from '../TagLabel';
import type { Tag } from '../../types/issue';

interface TagManagerProps {
  open: boolean;
  onClose: () => void;
}

type Draft = { name: string; color: string };

// Rename and recolor tags. Each row saves on its own, so a typo in one row
// never blocks the rest.
const TagManager: React.FC<TagManagerProps> = ({ open, onClose }) => {
  const queryClient = useQueryClient();
  const { data: tags = [], isLoading } = useQuery({ queryKey: ['tags'], queryFn: listTags, enabled: open });
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});

  const draftOf = (t: Tag): Draft => drafts[t.id] ?? { name: t.name, color: t.color ?? '#e1e4e8' };
  const setDraft = (t: Tag, patch: Partial<Draft>) =>
    setDrafts((d) => ({ ...d, [t.id]: { ...draftOf(t), ...patch } }));

  const save = useMutation({
    mutationFn: ({ tag, draft }: { tag: Tag; draft: Draft }) =>
      updateTag(tag.id, {
        name: draft.name.trim() !== tag.name ? draft.name.trim() : undefined,
        color: draft.color !== tag.color ? draft.color : undefined,
      }),
    onSuccess: (_, { tag }) => {
      message.success('标签已更新');
      setDrafts((d) => {
        const rest = { ...d };
        delete rest[tag.id];
        return rest;
      });
      queryClient.invalidateQueries({ queryKey: ['tags'] });
      // Labels are embedded in issue responses, so refresh those too.
      queryClient.invalidateQueries({ queryKey: ['issues'] });
      queryClient.invalidateQueries({ queryKey: ['issue'] });
    },
    onError: (err: unknown) => {
      const apiError = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      message.error(apiError || '更新失败');
    },
  });

  const columns = [
    {
      title: '预览',
      width: 140,
      render: (_: unknown, t: Tag) => <TagLabel tag={draftOf(t)} />,
    },
    {
      title: '名称',
      render: (_: unknown, t: Tag) => (
        <Input size="small" value={draftOf(t).name} onChange={(e) => setDraft(t, { name: e.target.value })} />
      ),
    },
    {
      title: '颜色',
      width: 160,
      render: (_: unknown, t: Tag) => (
        <ColorPicker
          size="small"
          disabledAlpha
          showText
          value={draftOf(t).color}
          onChangeComplete={(c) => setDraft(t, { color: c.toHexString() })}
        />
      ),
    },
    {
      title: '',
      width: 80,
      render: (_: unknown, t: Tag) => {
        const d = draftOf(t);
        const dirty = d.name.trim() !== t.name || d.color !== t.color;
        return (
          <Button
            size="small"
            type="primary"
            disabled={!dirty || !d.name.trim()}
            loading={save.isPending && save.variables?.tag.id === t.id}
            onClick={() => save.mutate({ tag: t, draft: d })}
          >
            保存
          </Button>
        );
      },
    },
  ];

  return (
    <Modal title="管理标签" open={open} onCancel={onClose} footer={null} width={640}>
      <Space direction="vertical" style={{ width: '100%' }}>
        <Table rowKey="id" size="small" columns={columns} dataSource={tags} loading={isLoading} pagination={{ pageSize: 10 }} />
      </Space>
    </Modal>
  );
};

export default TagManager;
