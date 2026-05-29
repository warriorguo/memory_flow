import client from './client';

export interface SyncResult {
  applied: number;
  skipped: number;
  errors?: string[];
}

// Sync runs against the standalone binary's local convenience endpoints, which
// perform the outbound request to the remote server. Allow a long timeout since
// a full dataset transfer may take a while.
const SYNC_TIMEOUT = 5 * 60 * 1000;

export const syncPush = async (serverUrl: string, token?: string): Promise<SyncResult> => {
  const { data } = await client.post<SyncResult>(
    '/sync/push',
    { server_url: serverUrl, token: token || undefined },
    { timeout: SYNC_TIMEOUT }
  );
  return data;
};

export const syncPull = async (serverUrl: string, token?: string): Promise<SyncResult> => {
  const { data } = await client.post<SyncResult>(
    '/sync/pull',
    { server_url: serverUrl, token: token || undefined },
    { timeout: SYNC_TIMEOUT }
  );
  return data;
};
