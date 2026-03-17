import React from 'react';
import { Select, Tag as AntTag } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { listTags } from '../../api/tag';
import type { Tag } from '../../types/issue';

interface TagSelectorProps {
  value?: string[];
  onChange?: (value: string[]) => void;
}

const TagSelector: React.FC<TagSelectorProps> = ({ value, onChange }) => {
  const { data: tags = [] } = useQuery({ queryKey: ['tags'], queryFn: listTags });

  return (
    <Select
      mode="multiple"
      placeholder="选择标签"
      value={value}
      onChange={onChange}
      options={tags.map((t: Tag) => ({ label: t.name, value: t.id }))}
      tagRender={({ label, closable, onClose }) => (
        <AntTag closable={closable} onClose={onClose} style={{ marginRight: 3 }}>
          {label}
        </AntTag>
      )}
    />
  );
};

export default TagSelector;
