package service

import "regexp"

// AssetRefScheme is the prefix an issue description uses to point at one of its
// own assets: `asset:enemy_ref.png`, or in markdown, `![art](asset:enemy_ref.png)`.
// References resolve within the issue that owns the description — a filename is
// only unique per issue, so cross-issue references would be ambiguous.
const AssetRefScheme = "asset:"

// assetRefPattern matches a reference and captures the filename. The charset is
// letters (including CJK), digits, and the punctuation that shows up inside
// filenames — so a reference ends at prose punctuation, whether ASCII ("see
// asset:spec.md.") or full-width ("参考 asset:art.png，音效…"). A filename
// containing a space has to be written in markdown link form.
var assetRefPattern = regexp.MustCompile(`asset:([\p{L}\p{N}._\-+~%@]+)`)

// trailingPunctuation is stripped from a captured filename when the raw capture
// does not name a real asset — prose runs references into sentences.
var trailingPunctuation = regexp.MustCompile(`[.,;:!?]+$`)

// AssetRefs returns the filenames a description references, in order and
// deduplicated. Names are returned exactly as written; matching them against
// real assets is the caller's job.
func AssetRefs(description string) []string {
	seen := map[string]bool{}
	var refs []string
	for _, match := range assetRefPattern.FindAllStringSubmatch(description, -1) {
		name := match[1]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		refs = append(refs, name)
	}
	return refs
}

// ResolveAssetRef picks the asset a reference names, trying the raw capture
// first and then the capture with trailing punctuation removed. It returns the
// resolved filename and whether anything matched.
func ResolveAssetRef(name string, known map[string]bool) (string, bool) {
	if known[name] {
		return name, true
	}
	trimmed := trailingPunctuation.ReplaceAllString(name, "")
	if trimmed != name && known[trimmed] {
		return trimmed, true
	}
	return name, false
}

// ReferencedIn reports whether the description points at the given filename.
func ReferencedIn(description, filename string) bool {
	known := map[string]bool{filename: true}
	for _, ref := range AssetRefs(description) {
		if resolved, ok := ResolveAssetRef(ref, known); ok && resolved == filename {
			return true
		}
	}
	return false
}
