package model

import (
	"reflect"
	"testing"
)

func TestDecodeCICDLinks(t *testing.T) {
	str := func(s string) *string { return &s }
	cases := []struct {
		name     string
		raw, url *string
		want     []CICDLink
	}{
		{"null column, no url", nil, nil, []CICDLink{}},
		{"null column, legacy url", nil, str("https://x"), []CICDLink{{URL: "https://x"}}},
		{"empty list, empty url", str("[]"), str(""), []CICDLink{}},
		{"stored list wins", str(`[{"label":"a","url":"https://a"}]`), str("https://a"),
			[]CICDLink{{Label: "a", URL: "https://a"}}},
		{"empty list, stale url", str("[]"), str("https://x"), []CICDLink{{URL: "https://x"}}},
	}
	for _, c := range cases {
		if got := DecodeCICDLinks(c.raw, c.url); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestEncodeCICDLinksNilIsNull(t *testing.T) {
	if EncodeCICDLinks(nil) != nil {
		t.Error("nil links should encode as NULL")
	}
	if got := EncodeCICDLinks([]CICDLink{}); got == nil || *got != "[]" {
		t.Errorf("empty links = %v, want []", got)
	}
}
