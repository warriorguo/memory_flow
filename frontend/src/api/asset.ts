import client from './client';
import type { IssueAsset } from '../types/asset';
import type { PaginatedResponse, ApiResponse } from '../types/common';

/**
 * assetUrl is the browser-facing address of an asset's bytes — usable directly
 * as an <img>, <video> or <audio> source. Downloads add ?download=1, which
 * flips the server's Content-Disposition to an attachment.
 */
export const assetUrl = (issueKey: string, filename: string, download = false): string => {
  const path = `/api/v1/issues/${encodeURIComponent(issueKey)}/assets/${encodeURIComponent(filename)}`;
  return download ? `${path}?download=1` : path;
};

export const listAssets = async (issueKey: string): Promise<IssueAsset[]> => {
  const { data } = await client.get<PaginatedResponse<IssueAsset>>(`/issues/${issueKey}/assets`);
  return data.data;
};

/**
 * uploadAsset attaches a file. Without overwrite the server answers 409 for a
 * name that is already taken, which the caller turns into a confirm-then-replace
 * prompt rather than clobbering someone's file silently.
 */
export const uploadAsset = async (
  issueKey: string,
  file: File,
  options: { overwrite?: boolean; onProgress?: (percent: number) => void } = {},
): Promise<IssueAsset> => {
  const form = new FormData();
  form.append('file', file);

  const { data } = await client.post<ApiResponse<IssueAsset>>(
    `/issues/${issueKey}/assets`,
    form,
    {
      params: options.overwrite ? { overwrite: 'true' } : undefined,
      // Uploads move far more data than a JSON call, so the client's default
      // timeout would abort a large file mid-flight.
      timeout: 120000,
      onUploadProgress: (event) => {
        if (options.onProgress && event.total) {
          options.onProgress(Math.round((event.loaded / event.total) * 100));
        }
      },
    },
  );
  return data.data;
};

export const replaceAsset = async (issueKey: string, filename: string, file: File): Promise<IssueAsset> => {
  const form = new FormData();
  form.append('file', file);

  const { data } = await client.put<ApiResponse<IssueAsset>>(
    `/issues/${issueKey}/assets/${encodeURIComponent(filename)}`,
    form,
    { timeout: 120000 },
  );
  return data.data;
};

export const deleteAsset = async (issueKey: string, filename: string): Promise<void> => {
  await client.delete(`/issues/${issueKey}/assets/${encodeURIComponent(filename)}`);
};

/** fetchAssetText loads a text asset's content for the preview pane. */
export const fetchAssetText = async (issueKey: string, filename: string): Promise<string> => {
  const { data } = await client.get<string>(`/issues/${issueKey}/assets/${encodeURIComponent(filename)}`, {
    responseType: 'text',
    transformResponse: [(raw) => raw],
  });
  return data;
};
