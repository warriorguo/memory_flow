import client from './client';
import { readerId } from '../types/comment';
import type { CommentThread, CreateCommentRequest, IssueComment, CommentSummary } from '../types/comment';
import type { ApiResponse } from '../types/common';

/**
 * listComments returns the thread together with its unread summary, both
 * computed for the current reader — one request, so a badge and a list never
 * disagree.
 */
export const listComments = async (issueKey: string): Promise<CommentThread> => {
  const { data } = await client.get<{ data: IssueComment[]; summary: CommentSummary }>(
    `/issues/${issueKey}/comments`,
    { params: { reader: readerId() } },
  );
  return { comments: data.data ?? [], summary: data.summary };
};

export const createComment = async (issueKey: string, req: CreateCommentRequest): Promise<IssueComment> => {
  const { data } = await client.post<ApiResponse<IssueComment>>(`/issues/${issueKey}/comments`, {
    author_id: readerId(),
    ...req,
  });
  return data.data;
};

/** markCommentsRead clears the unread flag for everything currently on the issue. */
export const markCommentsRead = async (issueKey: string): Promise<CommentSummary> => {
  const { data } = await client.post<ApiResponse<CommentSummary>>(
    `/issues/${issueKey}/comments/read`,
    { reader: readerId() },
  );
  return data.data;
};

export const deleteComment = async (issueKey: string, commentId: string): Promise<void> => {
  await client.delete(`/issues/${issueKey}/comments/${commentId}`);
};
