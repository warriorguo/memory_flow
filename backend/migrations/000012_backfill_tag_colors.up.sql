-- Every tag gets a color (MF-34): lists render tags as colored labels, and a
-- blank one would show as nothing. Backfill tags created without a color from
-- model.TagPalette, at slot (first code point + length) % 12 — the same
-- formula as model.PaletteIndex. Keep the palette order in sync with tag.go.
UPDATE tags
SET color = (ARRAY['#d73a4a', '#0075ca', '#a2eeef', '#7057ff', '#008672', '#e4e669', '#d876e3', '#fbca04', '#0e8a16', '#b60205', '#1d76db', '#f9d0c4'])[1 + ((ascii(name) + length(name)) % 12)]
WHERE color IS NULL OR color = '';
