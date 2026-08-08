import React from 'react';
import ReactMarkdown, { defaultUrlTransform } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import 'highlight.js/styles/github.css';
import './markdown.css';
import { resolveAssetRefs, MISSING_ASSET_SRC } from './assetRefs';
import type { AssetRefContext } from './assetRefs';

interface MarkdownProps {
  source?: string | null;
  empty?: React.ReactNode;
  /**
   * Supply the issue and its attachments to make `asset:<filename>` references
   * in the text resolve to the real files.
   */
  assetContext?: AssetRefContext;
}

const isMissing = (src?: string) => !!src && src.startsWith(MISSING_ASSET_SRC);

/**
 * react-markdown hands each component the mdast node it came from. It must not
 * be spread onto a DOM element, so strip it before forwarding the rest.
 */
const withoutNode = <T extends object>(props: T & { node?: unknown }) => {
  const { node, ...rest } = props;
  void node;
  return rest;
};

/** The name a marker src was built from, for the warning text. */
const missingName = (src: string) => src.slice(MISSING_ASSET_SRC.length);

/**
 * BrokenRef makes an unresolved asset: reference visible. Silently dropping it
 * would leave the reader believing a file exists somewhere.
 */
const BrokenRef: React.FC<{ name: string }> = ({ name }) => (
  <span
    style={{ color: '#cf1322', border: '1px dashed #cf1322', borderRadius: 4, padding: '0 6px' }}
    title="该 issue 没有这个附件"
  >
    ⚠ asset:{name}
  </span>
);

const Markdown: React.FC<MarkdownProps> = ({ source, empty = '-', assetContext }) => {
  if (!source || !source.trim()) {
    return <>{empty}</>;
  }

  const text = assetContext ? resolveAssetRefs(source, assetContext) : source;

  return (
    <div className="mf-markdown">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeHighlight]}
        // Keep the marker scheme intact; the default transform would strip an
        // unknown protocol and leave an empty src.
        urlTransform={(url) => (url.startsWith(MISSING_ASSET_SRC) ? url : defaultUrlTransform(url))}
        components={{
          a: ({ href, children, ...props }) =>
            isMissing(href) ? (
              <BrokenRef name={missingName(href!)} />
            ) : (
              <a {...withoutNode(props)} href={href} target="_blank" rel="noreferrer noopener">
                {children}
              </a>
            ),
          // An asset reference renders as markdown image syntax whatever its
          // kind, so video and audio become players here rather than a broken
          // <img>. The URL is the same one the API serves with Range support.
          img: ({ src, alt, ...props }) => {
            const url = typeof src === 'string' ? src : '';
            if (isMissing(url)) return <BrokenRef name={missingName(url)} />;
            if (/\.(mp4|webm|mov|m4v)(\?|$)/i.test(url)) {
              return <video src={url} controls style={{ maxWidth: '100%' }} />;
            }
            if (/\.(mp3|wav|ogg|m4a|flac)(\?|$)/i.test(url)) {
              return <audio src={url} controls style={{ width: '100%' }} />;
            }
            return <img {...withoutNode(props)} src={url} alt={alt} style={{ maxWidth: '100%' }} />;
          },
        }}
      >
        {text}
      </ReactMarkdown>
    </div>
  );
};

export default Markdown;
