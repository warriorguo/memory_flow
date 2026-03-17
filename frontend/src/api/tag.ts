import client from './client';
import type { Tag } from '../types/issue';
import type { ApiResponse } from '../types/common';

export const listTags = async (): Promise<Tag[]> => {
  const { data } = await client.get<ApiResponse<Tag[]>>('/tags');
  return data.data;
};

export const createTag = async (name: string, color?: string): Promise<Tag> => {
  const { data } = await client.post<ApiResponse<Tag>>('/tags', { name, color });
  return data.data;
};
