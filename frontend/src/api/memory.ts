import client from './client';
import type { Memory, CreateMemoryRequest, UpdateMemoryRequest, MemoryFilter } from '../types/memory';
import type { PaginatedResponse, ApiResponse } from '../types/common';

export const listMemories = async (filter: MemoryFilter = {}): Promise<PaginatedResponse<Memory>> => {
  const { data } = await client.get('/memories', { params: filter });
  return data;
};

export const getMemory = async (id: string): Promise<Memory> => {
  const { data } = await client.get<ApiResponse<Memory>>(`/memories/${id}`);
  return data.data;
};

export const createMemory = async (req: CreateMemoryRequest): Promise<Memory> => {
  const { data } = await client.post<ApiResponse<Memory>>('/memories', req);
  return data.data;
};

export const updateMemory = async (id: string, req: UpdateMemoryRequest): Promise<Memory> => {
  const { data } = await client.put<ApiResponse<Memory>>(`/memories/${id}`, req);
  return data.data;
};

export const deleteMemory = async (id: string): Promise<void> => {
  await client.delete(`/memories/${id}`);
};

export const addTagToMemory = async (memoryId: string, tagId: string): Promise<void> => {
  await client.post(`/memories/${memoryId}/tags`, { tag_id: tagId });
};

export const removeTagFromMemory = async (memoryId: string, tagId: string): Promise<void> => {
  await client.delete(`/memories/${memoryId}/tags/${tagId}`);
};
