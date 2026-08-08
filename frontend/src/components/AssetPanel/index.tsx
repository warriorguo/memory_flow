import React, { useRef, useState } from 'react';
import axios from 'axios';
import { Card, Table, Button, Upload, Modal, Space, Popconfirm, Spin, Empty, message } from 'antd';
import {
  FileImageOutlined,
  VideoCameraOutlined,
  SoundOutlined,
  FileTextOutlined,
  FileOutlined,
  InboxOutlined,
  DownloadOutlined,
  EyeOutlined,
  SwapOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { listAssets, uploadAsset, replaceAsset, deleteAsset, assetUrl, fetchAssetText } from '../../api/asset';
import { assetKind, formatSize } from '../../types/asset';
import type { IssueAsset } from '../../types/asset';
import Markdown from '../Markdown';

const KIND_ICONS: Record<string, React.ReactNode> = {
  image: <FileImageOutlined />,
  video: <VideoCameraOutlined />,
  audio: <SoundOutlined />,
  text: <FileTextOutlined />,
  other: <FileOutlined />,
};

interface AssetPanelProps {
  /** Issue key (OZX-12) or UUID — the API accepts either. */
  issueKey: string;
}

/**
 * AssetPanel is the issue's attachment drawer: drag in reference art, preview
 * it in place, hand it to whoever implements the issue, and replace it when the
 * source file is revised.
 */
const AssetPanel: React.FC<AssetPanelProps> = ({ issueKey }) => {
  const queryClient = useQueryClient();
  const [preview, setPreview] = useState<IssueAsset | null>(null);
  // The row whose "replace" button opened the file picker.
  const replacing = useRef<string | null>(null);
  const replaceInput = useRef<HTMLInputElement>(null);

  const { data: assets = [], isLoading } = useQuery({
    queryKey: ['assets', issueKey],
    queryFn: () => listAssets(issueKey),
    enabled: !!issueKey,
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['assets', issueKey] });

  const uploadMutation = useMutation({
    mutationFn: ({ file, overwrite }: { file: File; overwrite?: boolean }) =>
      uploadAsset(issueKey, file, { overwrite }),
    onSuccess: (asset) => {
      message.success(`已上传 ${asset.filename}`);
      refresh();
    },
  });

  const replaceMutation = useMutation({
    mutationFn: ({ filename, file }: { filename: string; file: File }) =>
      replaceAsset(issueKey, filename, file),
    onSuccess: (asset) => {
      message.success(`已替换 ${asset.filename}`);
      refresh();
    },
    onError: () => message.error('替换失败'),
  });

  const deleteMutation = useMutation({
    mutationFn: (filename: string) => deleteAsset(issueKey, filename),
    onSuccess: () => {
      message.success('已删除');
      refresh();
    },
    onError: () => message.error('删除失败'),
  });

  /**
   * A name that is already taken comes back as 409. Rather than failing or
   * silently overwriting, ask — replacing a revised file under the same name is
   * the common case, but it should be the user's call.
   */
  const handleUpload = async (file: File) => {
    try {
      await uploadMutation.mutateAsync({ file });
    } catch (err) {
      const response = axios.isAxiosError(err) ? err.response : undefined;
      if (response?.status === 409) {
        Modal.confirm({
          title: `${file.name} 已存在`,
          content: '是否用新文件替换现有内容？',
          okText: '替换',
          cancelText: '取消',
          onOk: () => uploadMutation.mutateAsync({ file, overwrite: true }).catch(() => message.error('替换失败')),
        });
        return;
      }
      message.error(response?.data?.error || '上传失败');
    }
  };

  const openReplacePicker = (filename: string) => {
    replacing.current = filename;
    replaceInput.current?.click();
  };

  const onReplaceFileChosen = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    const filename = replacing.current;
    // Reset so choosing the same file twice in a row still fires a change event.
    event.target.value = '';
    replacing.current = null;
    if (file && filename) {
      replaceMutation.mutate({ filename, file });
    }
  };

  const columns = [
    {
      title: '文件名',
      dataIndex: 'filename',
      render: (filename: string, asset: IssueAsset) => (
        <Space>
          {KIND_ICONS[assetKind(asset)]}
          <a onClick={() => setPreview(asset)}>{filename}</a>
        </Space>
      ),
    },
    { title: '类型', dataIndex: 'mime_type', width: 180, ellipsis: true },
    {
      title: '大小',
      dataIndex: 'size_bytes',
      width: 100,
      render: (bytes: number) => formatSize(bytes),
    },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      width: 160,
      render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '操作',
      width: 220,
      render: (_: unknown, asset: IssueAsset) => (
        <Space size="small">
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setPreview(asset)}>
            预览
          </Button>
          <Button
            type="link"
            size="small"
            icon={<DownloadOutlined />}
            href={assetUrl(issueKey, asset.filename, true)}
            target="_blank"
          >
            下载
          </Button>
          <Button type="link" size="small" icon={<SwapOutlined />} onClick={() => openReplacePicker(asset.filename)}>
            替换
          </Button>
          <Popconfirm
            title={`删除 ${asset.filename}？`}
            description="描述中对该文件的引用将失效。"
            okText="删除"
            cancelText="取消"
            onConfirm={() => deleteMutation.mutate(asset.filename)}
          >
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Card title="附件" style={{ marginTop: 16 }}>
      <Upload.Dragger
        multiple
        showUploadList={false}
        beforeUpload={(file) => {
          handleUpload(file as File);
          // Uploading is handled here, so keep antd's own uploader out of it.
          return Upload.LIST_IGNORE;
        }}
        style={{ marginBottom: 16 }}
      >
        <p className="ant-upload-drag-icon">
          <InboxOutlined />
        </p>
        <p className="ant-upload-text">拖拽文件到此处，或点击选择</p>
        <p className="ant-upload-hint">图片、视频、音频、日志、设计稿都可以；描述中可用 asset:文件名 引用</p>
      </Upload.Dragger>

      {isLoading ? (
        <Spin />
      ) : assets.length === 0 ? (
        <Empty description="暂无附件" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <Table
          rowKey="id"
          size="small"
          pagination={false}
          columns={columns}
          dataSource={assets}
          loading={replaceMutation.isPending || deleteMutation.isPending}
        />
      )}

      {/* One shared input drives every row's replace button. */}
      <input type="file" ref={replaceInput} style={{ display: 'none' }} onChange={onReplaceFileChosen} />

      <AssetPreview issueKey={issueKey} asset={preview} onClose={() => setPreview(null)} />
    </Card>
  );
};

interface AssetPreviewProps {
  issueKey: string;
  asset: IssueAsset | null;
  onClose: () => void;
}

/**
 * AssetPreview renders what the browser can render. Video and audio stream from
 * the same URL the API serves with Range support, so scrubbing works without
 * downloading the whole file.
 */
const AssetPreview: React.FC<AssetPreviewProps> = ({ issueKey, asset, onClose }) => {
  const kind = asset ? assetKind(asset) : 'other';

  const { data: text, isLoading: textLoading } = useQuery({
    queryKey: ['assetText', issueKey, asset?.filename],
    queryFn: () => fetchAssetText(issueKey, asset!.filename),
    enabled: !!asset && kind === 'text',
  });

  if (!asset) return null;
  const src = assetUrl(issueKey, asset.filename);
  const isMarkdown = /\.(md|markdown)$/i.test(asset.filename);

  return (
    <Modal
      title={asset.filename}
      open
      onCancel={onClose}
      width={800}
      footer={
        <Button type="primary" href={assetUrl(issueKey, asset.filename, true)} target="_blank">
          下载
        </Button>
      }
    >
      {kind === 'image' && <img src={src} alt={asset.filename} style={{ maxWidth: '100%' }} />}
      {kind === 'video' && <video src={src} controls style={{ maxWidth: '100%' }} />}
      {kind === 'audio' && <audio src={src} controls style={{ width: '100%' }} />}
      {kind === 'text' &&
        (textLoading ? (
          <Spin />
        ) : isMarkdown ? (
          <Markdown source={text} />
        ) : (
          <pre style={{ maxHeight: 480, overflow: 'auto', whiteSpace: 'pre-wrap' }}>{text}</pre>
        ))}
      {kind === 'other' && (
        <Empty description={`${asset.mime_type} 无法预览，请下载查看`} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      )}
    </Modal>
  );
};

export default AssetPanel;
