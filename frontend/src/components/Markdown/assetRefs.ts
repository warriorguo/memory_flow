import { assetUrl } from '../../api/asset';
import { assetKind } from '../../types/asset';
import type { IssueAsset } from '../../types/asset';

/**
 * A description points at its own attachments with `asset:<filename>` — bare in
 * prose, or as a markdown target: `![art](asset:enemy_ref.png)`. References
 * resolve within the issue that owns the description, because a filename is only
 * unique per issue.
 *
 * The charset mirrors the backend's (see service/asset_ref.go): letters
 * including CJK, digits, and the punctuation that appears inside filenames — so
 * a reference ends at prose punctuation, ASCII or full-width. A filename with a
 * space has to be written in markdown link form.
 */
const ASSET_REF = /asset:([\p{L}\p{N}._\-+~%@]+)/gu;
const TRAILING_PUNCTUATION = /[.,;:!?]+$/;

/** Marker src for a reference that names no existing asset. */
export const MISSING_ASSET_SRC = 'mf-missing-asset:';

export interface AssetRefContext {
  issueKey: string;
  assets: IssueAsset[];
}

/**
 * resolveAssetRefs rewrites `asset:` references into real URLs before the
 * markdown is parsed, so images and players get a working source and unresolved
 * names become a visible marker instead of vanishing into a dead link.
 *
 * Bare references also gain markdown syntax: previewable kinds become images
 * (the renderer turns video and audio into players), everything else a link.
 */
export const resolveAssetRefs = (source: string, ctx: AssetRefContext): string => {
  const byName = new Map(ctx.assets.map((a) => [a.filename, a]));

  return source.replace(ASSET_REF, (match, rawName: string, offset: number) => {
    // A reference already inside markdown syntax — ](asset:x) — only needs its
    // target swapped; the surrounding syntax decides how it renders.
    const inMarkdownTarget = offset > 0 && source[offset - 1] === '(';

    let name = rawName;
    let asset = byName.get(name);
    let trailing = '';
    if (!asset) {
      const trimmed = name.replace(TRAILING_PUNCTUATION, '');
      if (trimmed !== name && byName.get(trimmed)) {
        trailing = name.slice(trimmed.length);
        name = trimmed;
        asset = byName.get(name);
      }
    }

    if (!asset) {
      return inMarkdownTarget ? `${MISSING_ASSET_SRC}${rawName}` : `[⚠ ${match}](${MISSING_ASSET_SRC}${rawName})`;
    }

    const url = assetUrl(ctx.issueKey, name);
    if (inMarkdownTarget) {
      return url;
    }
    const kind = assetKind(asset);
    const embed = kind === 'image' || kind === 'video' || kind === 'audio';
    return `${embed ? '!' : ''}[${name}](${url})${trailing}`;
  });
};
