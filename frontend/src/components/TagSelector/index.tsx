import React, { useState } from 'react';
import { Select, message } from 'antd';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { listTags, createTag } from '../../api/tag';
import TagLabel from '../TagLabel';
import type { Tag } from '../../types/issue';

interface TagSelectorProps {
  value?: string[];
  onChange?: (value: string[]) => void;
  // allowCreate lets typing a new name create the tag (forms); filters leave it off.
  allowCreate?: boolean;
  placeholder?: string;
  style?: React.CSSProperties;
}

// Picks tags by id. With allowCreate, a typed name that matches no tag is
// created on the spot (the server assigns it a palette color).
const TagSelector: React.FC<TagSelectorProps> = ({ value, onChange, allowCreate, placeholder, style }) => {
  const queryClient = useQueryClient();
  const { data: tags = [] } = useQuery({ queryKey: ['tags'], queryFn: listTags });
  const [creating, setCreating] = useState(false);
  const byId = new Map(tags.map((t: Tag) => [t.id, t]));

  const handleChange = async (next: string[]) => {
    if (!allowCreate) {
      onChange?.(next);
      return;
    }
    const ids: string[] = [];
    let created = false;
    setCreating(true);
    try {
      for (const v of next) {
        if (byId.has(v)) {
          ids.push(v);
          continue;
        }
        const name = v.trim();
        if (!name) continue;
        const existing = tags.find((t) => t.name.toLowerCase() === name.toLowerCase());
        if (existing) {
          ids.push(existing.id);
          continue;
        }
        const tag = await createTag(name);
        ids.push(tag.id);
        created = true;
      }
    } catch {
      message.error('创建标签失败');
    } finally {
      setCreating(false);
    }
    if (created) await queryClient.invalidateQueries({ queryKey: ['tags'] });
    onChange?.(Array.from(new Set(ids)));
  };

  return (
    <Select
      mode={allowCreate ? 'tags' : 'multiple'}
      placeholder={placeholder ?? (allowCreate ? '选择或输入新标签' : '选择标签')}
      value={value}
      onChange={handleChange}
      loading={creating}
      allowClear
      style={style}
      optionFilterProp="label"
      options={tags.map((t: Tag) => ({ label: t.name, value: t.id }))}
      optionRender={(opt) => {
        const tag = byId.get(String(opt.value));
        return tag ? <TagLabel tag={tag} /> : opt.label;
      }}
      tagRender={({ value: v, label, closable, onClose }) => {
        const tag = byId.get(String(v));
        return (
          <span style={{ marginRight: 3, display: 'inline-flex', alignItems: 'center' }}>
            <TagLabel tag={tag ?? { name: String(label), color: undefined }} />
            {closable && (
              <span
                onMouseDown={(e) => e.preventDefault()}
                onClick={onClose}
                style={{ marginLeft: 2, cursor: 'pointer', color: '#999' }}
              >
                ×
              </span>
            )}
          </span>
        );
      }}
    />
  );
};

export default TagSelector;
