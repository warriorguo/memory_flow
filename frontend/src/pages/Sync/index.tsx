import React, { useState } from 'react';
import { Card, Input, Button, Space, Typography, Alert, Descriptions, message } from 'antd';
import { CloudUploadOutlined, CloudDownloadOutlined } from '@ant-design/icons';
import { syncPush, syncPull, type SyncResult } from '../../api/sync';

const { Paragraph, Text } = Typography;
const STORAGE_KEY = 'memory_flow.sync.server_url';
const TOKEN_KEY = 'memory_flow.sync.token';

const SyncPage: React.FC = () => {
  const [serverUrl, setServerUrl] = useState<string>(() => localStorage.getItem(STORAGE_KEY) || '');
  const [token, setToken] = useState<string>(() => localStorage.getItem(TOKEN_KEY) || '');
  const [busy, setBusy] = useState<'push' | 'pull' | null>(null);
  const [result, setResult] = useState<{ kind: 'push' | 'pull'; data: SyncResult } | null>(null);
  const [error, setError] = useState<string>('');

  const run = async (kind: 'push' | 'pull') => {
    const url = serverUrl.trim();
    if (!url) {
      message.warning('请先填写服务器地址');
      return;
    }
    localStorage.setItem(STORAGE_KEY, url);
    localStorage.setItem(TOKEN_KEY, token);
    setBusy(kind);
    setError('');
    setResult(null);
    try {
      const data = kind === 'push' ? await syncPush(url, token) : await syncPull(url, token);
      setResult({ kind, data });
      message.success(kind === 'push' ? '已推送到服务器' : '已从服务器拉取');
    } catch (e: any) {
      setError(e?.response?.data?.error || e?.message || '同步失败');
    } finally {
      setBusy(null);
    }
  };

  return (
    <div style={{ maxWidth: 720, margin: '0 auto' }}>
      <Card title="数据同步">
        <Paragraph type="secondary">
          在外出（无法访问家庭网络的 Memory Flow 服务器）时，本地以独立模式运行并保存数据。
          回到家后，可将本地数据 <Text strong>推送（合并）</Text> 到服务器，或从服务器
          <Text strong> 拉取（合并）</Text> 到本地。冲突按 <Text code>updated_at</Text> 取较新者（last-writer-wins）。
        </Paragraph>

        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Input
            addonBefore="服务器地址"
            placeholder="http://memory-flow.home.lan:8080"
            value={serverUrl}
            onChange={(e) => setServerUrl(e.target.value)}
            allowClear
          />

          <Input.Password
            addonBefore="同步令牌"
            placeholder="SYNC_TOKEN（服务器已开启鉴权时必填）"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            allowClear
          />

          <Space>
            <Button
              type="primary"
              icon={<CloudUploadOutlined />}
              loading={busy === 'push'}
              disabled={busy !== null}
              onClick={() => run('push')}
            >
              推送到服务器
            </Button>
            <Button
              icon={<CloudDownloadOutlined />}
              loading={busy === 'pull'}
              disabled={busy !== null}
              onClick={() => run('pull')}
            >
              从服务器拉取
            </Button>
          </Space>

          {error && <Alert type="error" showIcon message="同步失败" description={error} />}

          {result && (
            <Alert
              type={result.data.errors && result.data.errors.length ? 'warning' : 'success'}
              showIcon
              message={result.kind === 'push' ? '推送完成' : '拉取完成'}
              description={
                <Descriptions size="small" column={1} style={{ marginTop: 8 }}>
                  <Descriptions.Item label="已应用">{result.data.applied}</Descriptions.Item>
                  <Descriptions.Item label="已跳过">{result.data.skipped}</Descriptions.Item>
                  {result.data.errors && result.data.errors.length > 0 && (
                    <Descriptions.Item label="冲突/跳过明细">
                      <ul style={{ margin: 0, paddingLeft: 18 }}>
                        {result.data.errors.map((e, i) => (
                          <li key={i}><Text type="warning">{e}</Text></li>
                        ))}
                      </ul>
                    </Descriptions.Item>
                  )}
                </Descriptions>
              }
            />
          )}
        </Space>
      </Card>
    </div>
  );
};

export default SyncPage;
