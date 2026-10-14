import client from './client';
import type { IssueDependency } from '../types/dependency';
import type { ApiResponse } from '../types/common';

/** Every dependency the issue takes part in, on either side, with both ends summarized. */
export const listDependencies = async (issueKey: string): Promise<IssueDependency[]> => {
  const { data } = await client.get<ApiResponse<IssueDependency[]>>(`/issues/${issueKey}/dependencies`);
  return data.data;
};
