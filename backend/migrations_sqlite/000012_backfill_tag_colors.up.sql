-- Every tag gets a color (MF-34): lists render tags as colored labels, and a
-- blank one would show as nothing. Backfill tags created without a color from
-- model.TagPalette, at slot (first code point + length) % 12 — the same
-- formula as model.PaletteIndex. Keep the palette order in sync with tag.go.
UPDATE tags
SET color = CASE (unicode(name) + length(name)) % 12
    WHEN 0 THEN '#d73a4a'
    WHEN 1 THEN '#0075ca'
    WHEN 2 THEN '#a2eeef'
    WHEN 3 THEN '#7057ff'
    WHEN 4 THEN '#008672'
    WHEN 5 THEN '#e4e669'
    WHEN 6 THEN '#d876e3'
    WHEN 7 THEN '#fbca04'
    WHEN 8 THEN '#0e8a16'
    WHEN 9 THEN '#b60205'
    WHEN 10 THEN '#1d76db'
    WHEN 11 THEN '#f9d0c4'
END
WHERE color IS NULL OR color = '';
