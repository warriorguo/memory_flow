package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CICDLink is one CI/CD pipeline a project deploys through. A repo that ships
// several apps (e.g. a server and an admin console) records one link per app,
// told apart by Label. Label is empty for the link migrated from the legacy
// single cicd_url field.
type CICDLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// NormalizeCICDLinks trims every link and rejects ones without a URL or with a
// label used twice. It always returns a non-nil slice.
func NormalizeCICDLinks(links []CICDLink) ([]CICDLink, error) {
	out := make([]CICDLink, 0, len(links))
	seen := map[string]bool{}
	for _, l := range links {
		l.Label = strings.TrimSpace(l.Label)
		l.URL = strings.TrimSpace(l.URL)
		if l.URL == "" {
			return nil, fmt.Errorf("cicd link %q has no url", l.Label)
		}
		if l.Label != "" {
			if seen[l.Label] {
				return nil, fmt.Errorf("duplicate cicd link label %q", l.Label)
			}
			seen[l.Label] = true
		}
		out = append(out, l)
	}
	return out, nil
}

// CICDLinksFromURL is the legacy cicd_url expressed as a link list: one
// unlabeled link, or none for an empty URL.
func CICDLinksFromURL(url string) []CICDLink {
	url = strings.TrimSpace(url)
	if url == "" {
		return []CICDLink{}
	}
	return []CICDLink{{URL: url}}
}

// PrimaryCICDURL is the value the legacy cicd_url column holds for a link list:
// the first link's URL, or "" when there are none.
func PrimaryCICDURL(links []CICDLink) string {
	if len(links) == 0 {
		return ""
	}
	return links[0].URL
}

// EncodeCICDLinks serializes links for the cicd_links column. A nil slice
// encodes as NULL, which reads back through the legacy cicd_url.
func EncodeCICDLinks(links []CICDLink) *string {
	if links == nil {
		return nil
	}
	raw, _ := json.Marshal(links)
	s := string(raw)
	return &s
}

// DecodeCICDLinks reads the cicd_links column, falling back to the legacy
// cicd_url when the list is empty but the URL is not. That fallback covers rows
// written before cicd_links existed, including rows synced in from an older
// peer. Writers keep cicd_url equal to the first link (empty when there are no
// links), so the fallback never resurrects a list that was cleared.
func DecodeCICDLinks(raw, legacyURL *string) []CICDLink {
	var links []CICDLink
	if raw != nil && *raw != "" {
		_ = json.Unmarshal([]byte(*raw), &links)
	}
	if len(links) == 0 && legacyURL != nil {
		return CICDLinksFromURL(*legacyURL)
	}
	if links == nil {
		links = []CICDLink{}
	}
	return links
}
