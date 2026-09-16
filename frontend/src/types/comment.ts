export interface IssueComment {
  id: string;
  issue_id: string;
  author_id?: string | null;
  body: string;
  /** Whether the reader named in the request has yet to read this comment. */
  unread: boolean;
  created_at: string;
  updated_at: string;
}

/** The "you have mail" line for an issue. */
export interface CommentSummary {
  total: number;
  unread: number;
  last_comment_at?: string;
  last_author?: string;
}

export interface CommentThread {
  comments: IssueComment[];
  summary: CommentSummary;
}

export interface CreateCommentRequest {
  body: string;
  author_id?: string;
}

const READER_KEY = 'mf_reader';

/**
 * readerId names whoever is looking, which is what makes "unread" mean
 * anything. The API has no auth, so this is a convention: the signed-in
 * username when there is one, otherwise a per-browser id so a visitor's read
 * state does not merge with everyone else's.
 */
export const readerId = (): string => {
  try {
    const stored = localStorage.getItem('user');
    if (stored) {
      const user = JSON.parse(stored);
      if (user?.username) return user.username;
    }
  } catch {
    // A malformed or unreadable entry just means we fall back to the browser id.
  }
  try {
    const existing = localStorage.getItem(READER_KEY);
    if (existing) return existing;
    const generated = `web-${Math.random().toString(36).slice(2, 10)}`;
    localStorage.setItem(READER_KEY, generated);
    return generated;
  } catch {
    // Private mode with storage blocked: a stable-enough name beats crashing.
    return 'web';
  }
};
