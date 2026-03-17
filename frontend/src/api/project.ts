import client from './client';
import type { Project, CreateProjectRequest, UpdateProjectRequest, ProjectFilter } from '../types/project';
import type { PaginatedResponse, ApiResponse } from '../types/common';

export const listProjects = async (filter: ProjectFilter = {}): Promise<PaginatedResponse<Project>> => {
  const { data } = await client.get('/projects', { params: filter });
  return data;
};

export const getProject = async (id: string): Promise<Project> => {
  const { data } = await client.get<ApiResponse<Project>>(`/projects/${id}`);
  return data.data;
};

export const createProject = async (req: CreateProjectRequest): Promise<Project> => {
  const { data } = await client.post<ApiResponse<Project>>('/projects', req);
  return data.data;
};

export const updateProject = async (id: string, req: UpdateProjectRequest): Promise<Project> => {
  const { data } = await client.put<ApiResponse<Project>>(`/projects/${id}`, req);
  return data.data;
};

export const archiveProject = async (id: string): Promise<void> => {
  await client.delete(`/projects/${id}`);
};
