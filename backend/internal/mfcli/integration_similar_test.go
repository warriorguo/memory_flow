package mfcli

import (
	"strings"
	"testing"
)

// MF-35: the OZX-1956 / OZX-1958 case end to end — two issues filed a moment
// apart, worded differently, citing the same session dir and room id. The
// second create must still succeed without a TTY and list the first.
func TestIntegrationSimilarOnCreate(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("DUP", "Dedup")

	m.run("issue", "create", "DUP", "--type", "bug",
		"--title", "联机：错开后的进门落点压在坑沿，P2 一进窄走廊房间就坠落",
		"--desc", "遥测会话 `2026-11-11_172018`（P2），房间 story_3_f1_room_4_-1 的 Right 门，三次进门坠落。")
	m.run("issue", "create", "DUP", "--type", "bug", "--title", "商店价格多显示一位小数")

	second := "story 房 story_3_f1_room_4_-1 进门落点在虚空格：进门 0.04s 即 fall_begin"
	secondDesc := "会话目录 2026-11-11_172018，P2 从 Right 门进入后立即坠落。"

	// Check before filing.
	out := m.run("issue", "similar", "DUP", "--title", second, "--desc", secondDesc)
	m.contains(out, "likely", "DUP-1", "2026-11-11_172018", "story_3_f1_room_4_-1")
	m.omits(out, "DUP-2")

	// Filing anyway: created, with the hint after the usual line.
	out = m.run("issue", "create", "DUP", "--type", "bug", "--title", second, "--desc", secondDesc)
	if !strings.HasPrefix(out, "created DUP-3  [bug] P2 ") {
		t.Errorf("the created line must stay first and unchanged:\n%s", out)
	}
	m.contains(out, "Possible duplicates of DUP-3", "likely", "DUP-1")

	// --json keeps every issue field and adds similar_issues.
	data := m.json("issue", "create", "DUP", "--type", "bug", "--title", second, "--desc", secondDesc)
	if data["key"] != "DUP-4" || data["title"] != second {
		t.Errorf("issue fields missing from create response: %v", data)
	}
	// DUP-3 has the very same text, so it leads; DUP-1 follows.
	var keys []string
	for _, s := range data["similar_issues"].([]any) {
		keys = append(keys, s.(map[string]any)["key"].(string))
	}
	if strings.Join(keys, ",") != "DUP-3,DUP-1" {
		t.Errorf("similar_issues = %v, want DUP-3,DUP-1", keys)
	}

	// Checking an existing issue leaves itself out.
	out = m.run("issue", "similar", "DUP-1")
	m.contains(out, "DUP-3")
	m.omits(out, "DUP-1 ")

	// --not-dup: still created, the judgement goes into the history.
	out = m.run("issue", "create", "DUP", "--type", "bug", "--title", second, "--desc", secondDesc, "--not-dup")
	m.contains(out, "created DUP-5", "not a duplicate of DUP-")
	m.omits(out, "Possible duplicates")
	m.contains(m.run("issue", "history", "DUP-5"), "not_duplicate_of")

	// Nothing alike: no hint at all.
	out = m.run("issue", "create", "DUP", "--type", "requirement", "--title", "导出 PDF 报表")
	m.omits(out, "Possible duplicates")

	m.fails("issue", "similar", "DUP")
}
