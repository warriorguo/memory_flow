import React from 'react';
import type { Tag } from '../../types/issue';
import { tagTextColor } from './color';


const FALLBACK = '#e1e4e8';

interface TagLabelProps {
  tag: Pick<Tag, 'name' | 'color'>;
  onClick?: (e: React.MouseEvent) => void;
  active?: boolean;
  title?: string;
}

// A GitHub-style issue label: a rounded pill filled with the tag's color.
const TagLabel: React.FC<TagLabelProps> = ({ tag, onClick, active, title }) => {
  const bg = tag.color || FALLBACK;
  return (
    <span
      onClick={onClick}
      title={title}
      style={{
        display: 'inline-block',
        padding: '0 7px',
        marginLeft: 4,
        fontSize: 12,
        fontWeight: 500,
        lineHeight: '18px',
        borderRadius: 9,
        background: bg,
        color: tagTextColor(bg),
        whiteSpace: 'nowrap',
        verticalAlign: 'middle',
        cursor: onClick ? 'pointer' : undefined,
        boxShadow: active ? '0 0 0 2px #1677ff' : undefined,
      }}
    >
      {tag.name}
    </span>
  );
};

interface TagLabelsProps {
  tags?: Tag[];
  onTagClick?: (tag: Tag) => void;
  activeIds?: string[];
}

export const TagLabels: React.FC<TagLabelsProps> = ({ tags, onTagClick, activeIds }) => {
  if (!tags || tags.length === 0) return null;
  return (
    <>
      {tags.map((t) => (
        <TagLabel
          key={t.id}
          tag={t}
          active={activeIds?.includes(t.id)}
          title={onTagClick ? `按标签「${t.name}」筛选` : undefined}
          onClick={
            onTagClick
              ? (e) => {
                  e.stopPropagation();
                  onTagClick(t);
                }
              : undefined
          }
        />
      ))}
    </>
  );
};

export default TagLabel;
