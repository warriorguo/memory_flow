import React from 'react';
import { Button, Form, Input, Space, Tag } from 'antd';
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons';
import type { CICDLink } from '../../types/project';

// CicdLinksField edits a project's CI/CD links: one row per app the repo
// deploys, each with an optional label (e.g. server, admin) and a URL.
export const CicdLinksField: React.FC = () => (
  <Form.Item label="CI/CD 地址">
    <Form.List name="cicd_links">
      {(fields, { add, remove }) => (
        <>
          {fields.map(({ key, name }) => (
            <Space key={key} align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
              <Form.Item name={[name, 'label']} noStyle>
                <Input placeholder="标签，如 server" style={{ width: 140 }} />
              </Form.Item>
              <Form.Item
                name={[name, 'url']}
                noStyle
                rules={[{ required: true, whitespace: true, message: '请填写地址' }]}
              >
                <Input placeholder="https://…" style={{ width: 340 }} />
              </Form.Item>
              <MinusCircleOutlined onClick={() => remove(name)} />
            </Space>
          ))}
          <Button type="dashed" onClick={() => add({ label: '', url: '' })} icon={<PlusOutlined />}>
            添加 CI/CD 地址
          </Button>
        </>
      )}
    </Form.List>
  </Form.Item>
);

// CicdLinksView lists a project's CI/CD links, one per line.
export const CicdLinksView: React.FC<{ links?: CICDLink[] }> = ({ links }) => {
  if (!links || links.length === 0) return <>-</>;
  return (
    <Space direction="vertical" size={4}>
      {links.map((l, i) => (
        <span key={i}>
          {l.label && <Tag>{l.label}</Tag>}
          <a href={l.url} target="_blank" rel="noreferrer">{l.url}</a>
        </span>
      ))}
    </Space>
  );
};
