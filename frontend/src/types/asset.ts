export interface IssueAsset {
  id: string;
  issue_id: string;
  filename: string;
  mime_type: string;
  size_bytes: number;
  checksum: string;
  created_at: string;
  updated_at: string;
}

export type AssetKind = 'image' | 'video' | 'audio' | 'text' | 'other';

/**
 * Which preview an asset can support. Driven by the stored mime type, with an
 * extension fallback for the text-ish formats servers habitually report as
 * application/octet-stream (.md, .log, .json).
 */
export const assetKind = (asset: IssueAsset): AssetKind => {
  const mime = (asset.mime_type || '').toLowerCase();
  if (mime.startsWith('image/')) return 'image';
  if (mime.startsWith('video/')) return 'video';
  if (mime.startsWith('audio/')) return 'audio';
  if (mime.startsWith('text/') || mime.includes('json') || mime.includes('xml')) return 'text';

  const ext = asset.filename.toLowerCase().split('.').pop() || '';
  if (['md', 'markdown', 'txt', 'log', 'json', 'yaml', 'yml', 'csv'].includes(ext)) return 'text';
  return 'other';
};

export const formatSize = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB'];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(1)} ${units[unit]}`;
};
