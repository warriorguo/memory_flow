// Same rule as model.TagTextColor on the backend: perceived brightness
// (ITU-R BT.601) decides black or white text, so a label reads the same in the
// web UI and in `mf issues`.
export const tagTextColor = (background?: string): string => {
  const m = /^#?([0-9a-f]{6})$/i.exec(background ?? '');
  if (!m) return '#000000';
  const v = parseInt(m[1], 16);
  const r = (v >> 16) & 0xff;
  const g = (v >> 8) & 0xff;
  const b = v & 0xff;
  return (299 * r + 587 * g + 114 * b) / 1000 >= 150 ? '#000000' : '#ffffff';
};
