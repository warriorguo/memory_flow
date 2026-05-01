import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import 'highlight.js/styles/github.css';
import './markdown.css';

interface MarkdownProps {
  source?: string | null;
  empty?: React.ReactNode;
}

const Markdown: React.FC<MarkdownProps> = ({ source, empty = '-' }) => {
  if (!source || !source.trim()) {
    return <>{empty}</>;
  }

  return (
    <div className="mf-markdown">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeHighlight]}
        components={{
          a: ({ node: _node, ...props }) => (
            <a {...props} target="_blank" rel="noreferrer noopener" />
          ),
        }}
      >
        {source}
      </ReactMarkdown>
    </div>
  );
};

export default Markdown;
