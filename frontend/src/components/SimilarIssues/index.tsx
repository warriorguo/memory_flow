import React, { useEffect, useState } from 'react';
import { Alert, Checkbox, Form, List, Space, Tag, Typography } from 'antd';
import type { FormInstance } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { findSimilarIssues } from '../../api/issue';
import StatusTag from '../StatusTag';
import type { SimilarIssue } from '../../types/issue';

const { Text } = Typography;

/** Why a candidate was picked, in one line. */
const similarReasons = (s: SimilarIssue): string => {
  const why: string[] = [];
  if (s.shared_identifiers?.length) {
    const ids = s.shared_identifiers.slice(0, 4).join(', ');
    const more = s.shared_identifiers.length > 4 ? ` (+${s.shared_identifiers.length - 4})` : '';
    why.push(`共同标识：${ids}${more}`);
  }
  if (s.references) why.push(`引用了 ${s.key}`);
  if (s.title_similarity >= 0.3) why.push(`标题相似 ${Math.round(s.title_similarity * 100)}%`);
  return why.join('；');
};

export const SimilarIssueList: React.FC<{ items: SimilarIssue[]; onPick?: (s: SimilarIssue) => void }> = ({ items, onPick }) => (
  <List
    size="small"
    dataSource={items}
    renderItem={(s) => (
      <List.Item
        key={s.key}
        actions={onPick ? [<a key="pick" onClick={() => onPick(s)}>选择</a>] : undefined}
      >
        <Space direction="vertical" size={0} style={{ minWidth: 0 }}>
          <Space size={4} wrap>
            <Tag color={s.level === 'likely' ? 'red' : 'orange'}>{s.level === 'likely' ? '很可能' : '可能'}</Tag>
            <a href={`/issues/${s.key}`} target="_blank" rel="noreferrer">{s.key}</a>
            <StatusTag status={s.status} />
            <Text ellipsis style={{ maxWidth: 360 }}>{s.title}</Text>
          </Space>
          <Text type="secondary" style={{ fontSize: 12 }}>{similarReasons(s)}</Text>
        </Space>
      </List.Item>
    )}
  />
);

const useDebounced = <T,>(value: T, ms: number): T => {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
};

/**
 * Suspected duplicates of the draft in a create form (MF-35). It watches the
 * form's title and description, and when anything looks alike it lists it and
 * offers a "not a duplicate" checkbox bound to the form's not_duplicate field.
 * It never blocks submitting.
 */
const SimilarIssuesHint: React.FC<{ projectId: string; form: FormInstance }> = ({ projectId, form }) => {
  const title: string = Form.useWatch('title', form) ?? '';
  const description: string = Form.useWatch('description', form) ?? '';
  const type = Form.useWatch('type', form);
  const draft = useDebounced({ title: title.trim(), description: description.trim(), type }, 500);
  const enough = draft.title.length >= 4 || draft.description.length >= 8;

  const { data: similar = [] } = useQuery({
    queryKey: ['similar-issues', projectId, draft],
    queryFn: () => findSimilarIssues(projectId, draft),
    enabled: !!projectId && enough,
    staleTime: 10_000,
  });

  if (!enough || similar.length === 0) return null;
  return (
    <Alert
      type="warning"
      showIcon
      style={{ marginBottom: 16 }}
      message={`可能已有相同的工作项（${similar.length}）`}
      description={
        <>
          <SimilarIssueList items={similar} />
          <Form.Item name="not_duplicate" valuePropName="checked" style={{ marginBottom: 0 }}>
            <Checkbox>已确认不是重复，继续创建（会记入操作历史）</Checkbox>
          </Form.Item>
        </>
      }
    />
  );
};

export default SimilarIssuesHint;
