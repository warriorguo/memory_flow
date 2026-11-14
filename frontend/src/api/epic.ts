import client from './client';
import { readerId } from '../types/comment';
import type { ApiResponse } from '../types/common';
import type { EpicDetail, EpicProgress, EpicSummary, IssueMemory, SetEpicResult } from '../types/epic';

export const listEpics = async (projectId: string): Promise<EpicSummary[]> => {
  const { data } = await client.get<ApiResponse<EpicSummary[]>>(`/projects/${projectId}/epics`);
  return data.data;
};

export const getEpic = async (key: string): Promise<EpicDetail> => {
  const { data } = await client.get<ApiResponse<EpicDetail>>(`/epics/${key}`);
  return data.data;
};

export const getEpicProgress = async (key: string, days = 30): Promise<EpicProgress> => {
  const { data } = await client.get<ApiResponse<EpicProgress>>(`/epics/${key}/progress`, { params: { days } });
  return data.data;
};

/** setIssueEpic puts an issue into an epic, moves it, or (epic = '') takes it out. */
export const setIssueEpic = async (issueKey: string, epic: string): Promise<SetEpicResult> => {
  const { data } = await client.put<ApiResponse<SetEpicResult>>(`/issues/${issueKey}/epic`, { epic });
  return data.data;
};

/** listIssueMemories returns the issue's own memories and its epic's, with read state for this reader. */
export const listIssueMemories = async (issueKey: string): Promise<IssueMemory[]> => {
  const { data } = await client.get<ApiResponse<IssueMemory[]>>(`/issues/${issueKey}/memories`, {
    params: { reader: readerId() },
  });
  return data.data ?? [];
};

export const markMemoriesRead = async (memories: IssueMemory[]): Promise<void> => {
  if (memories.length === 0) return;
  await client.post('/memories/read', {
    reader: readerId(),
    memories: memories.map((m) => ({ memory_id: m.id, revision: m.revision })),
  });
};

export const promoteMemory = async (id: string, copy = false, force = false) => {
  const { data } = await client.post<ApiResponse<{ memory: { id: string; title: string }; epic: string }>>(
    `/memories/${id}/promote`,
    { copy, force },
  );
  return data.data;
};
