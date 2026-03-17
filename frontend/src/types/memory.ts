import type { MemoryType } from './common';
import type { Tag } from './issue';

export interface Memory {
  id: string;
  project_id?: string;
  type: MemoryType;
  title: string;
  content: string;
  source_object_type?: string;
  source_object_id?: string;
  creator_id?: string;
  tags?: Tag[];
  created_at: string;
  updated_at: string;
}

export interface CreateMemoryRequest {
  project_id?: string;
  type: MemoryType;
  title: string;
  content: string;
  source_object_type?: string;
  source_object_id?: string;
  tag_ids?: string[];
}

export interface UpdateMemoryRequest {
  title?: string;
  content?: string;
  type?: MemoryType;
}

export interface MemoryFilter {
  project_id?: string;
  type?: MemoryType;
  keyword?: string;
  source_object_type?: string;
  source_object_id?: string;
  tag?: string;
  page?: number;
  page_size?: number;
}
