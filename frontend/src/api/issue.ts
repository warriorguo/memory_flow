import client from './client';
import type { Issue, CreateIssueRequest, UpdateIssueRequest, IssueFilter, IssueHistory, ProgressSummary, TrendData } from '../types/issue';
import type { PaginatedResponse, ApiResponse } from '../types/common';

export const listIssues = async (projectId: string, filter: IssueFilter = {}): Promise<PaginatedResponse<Issue>> => {
  const { data } = await client.get(`/projects/${projectId}/issues`, { params: filter });
  return data;
};

export const getIssue = async (id: string): Promise<Issue> => {
  const { data } = await client.get<ApiResponse<Issue>>(`/issues/${id}`);
  return data.data;
};

export const createIssue = async (projectId: string, req: CreateIssueRequest): Promise<Issue> => {
  const { data } = await client.post<ApiResponse<Issue>>(`/projects/${projectId}/issues`, req);
  return data.data;
};

export const updateIssue = async (id: string, req: UpdateIssueRequest): Promise<Issue> => {
  const { data } = await client.put<ApiResponse<Issue>>(`/issues/${id}`, req);
  return data.data;
};

export const transitionIssueStatus = async (id: string, status: string): Promise<Issue> => {
  const { data } = await client.patch<ApiResponse<Issue>>(`/issues/${id}/status`, { status });
  return data.data;
};

export const getIssueHistory = async (id: string): Promise<IssueHistory[]> => {
  const { data } = await client.get<ApiResponse<IssueHistory[]>>(`/issues/${id}/history`);
  return data.data;
};

export const getProgressSummary = async (projectId: string): Promise<ProgressSummary> => {
  const { data } = await client.get<ApiResponse<ProgressSummary>>(`/projects/${projectId}/progress/summary`);
  return data.data;
};

export const getProgressTrend = async (projectId: string, days: number = 30): Promise<TrendData[]> => {
  const { data } = await client.get<ApiResponse<TrendData[]>>(`/projects/${projectId}/progress/trend`, { params: { days } });
  return data.data;
};

export const addTagToIssue = async (issueId: string, tagId: string): Promise<void> => {
  await client.post(`/issues/${issueId}/tags`, { tag_id: tagId });
};

export const removeTagFromIssue = async (issueId: string, tagId: string): Promise<void> => {
  await client.delete(`/issues/${issueId}/tags/${tagId}`);
};
