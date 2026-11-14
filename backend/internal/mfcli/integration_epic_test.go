package mfcli

import (
	"regexp"
	"testing"
)

var memoryIDRe = regexp.MustCompile(`memory ([0-9a-f-]{36})`)

func (m *mfTest) memoryID(out string) string {
	m.t.Helper()
	got := memoryIDRe.FindStringSubmatch(out)
	if got == nil {
		m.t.Fatalf("no memory id in %q", out)
	}
	return got[1]
}

// MF-39: epics end to end — the acceptance list of the requirement.
func TestIntegrationEpic(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("OZX", "Game")
	m.seedProject("OAE", "Art export")

	m.contains(m.run("epic", "create", "OZX", "--title", "无尽模式", "--desc", "无尽模式的总设计"), "created OZX-1  [epic]")
	m.contains(m.run("issue", "create", "OZX", "--type", "requirement", "--title", "无尽波次生成", "--epic", "OZX-1"), "in epic OZX-1")
	m.run("issue", "create", "OZX", "--type", "requirement", "--title", "无尽结算界面")
	m.run("issue", "create", "OAE", "--type", "requirement", "--title", "无尽模式图标导出")
	m.run("issue", "create", "OZX", "--type", "bug", "--title", "不在 epic 里的票")

	// 1. Three issues, one from another project; seen from mf, ratio right.
	m.contains(m.run("epic", "add", "OZX-1", "OZX-3", "OAE-1"), "OZX-3: added to OZX-1", "OAE-1: added to OZX-1")
	m.contains(m.run("epic", "add", "OZX-1", "OZX-3"), "OZX-3: already in OZX-1")
	show := m.run("epic", "show", "OZX-1")
	m.contains(show, "OZX-2", "OZX-3", "OAE-1", "Progress: 0/3 done (0%)")
	m.omits(show, "OZX-4")
	m.run("issue", "done", "OZX-2", "--force")
	m.contains(m.run("epic", "list", "OZX"), "OZX-1", "1/3", "33%")
	m.contains(m.run("issues", "--epic", "OZX-1"), "OZX-3", "OAE-1", "2 shown of 3 in epic OZX-1 (1/3 done)")
	byProject := m.run("issues", "OZX", "--epic", "OZX-1", "--all")
	m.contains(byProject, "OZX-2", "OZX-3")
	m.omits(byProject, "OAE-1", "OZX-4")
	m.contains(m.run("issue", "show", "OAE-1"), "epic      OZX-1 [todo] 无尽模式")
	m.contains(m.run("issue", "show", "OZX-1"), "Epic: 3 issue(s), 1/3 done (33%)")

	// Blocked: an epic issue waiting on an unfinished critical dependency.
	m.run("issue", "dep", "add", "OZX-3", "OAE-1", "--type", "depends_on", "--severity", "critical")
	show = m.run("epic", "show", "OZX-1")
	m.contains(show, "OZX-3 depends on OAE-1 (critical)", "OZX-3 [todo] waits on OAE-1 [todo]")
	m.contains(m.run("epic", "progress", "OZX-1", "--trend", "3"), "1/3 done", "waits on OAE-1", "DATE")

	// 2. An epic memory is visible from every issue in it, marked as such.
	death := m.memoryID(m.run("memory", "add", "--epic", "OZX-1", "--title", "联机死亡规则",
		"--content", "联机死亡＝倒地，到下一段复活"))
	a := m.run("--as", "bob", "issue", "show", "OAE-1")
	m.contains(a, "Epic memories (from OZX-1, 1 new or changed since bob last read them)", "联机死亡规则", "*new")
	m.omits(m.run("issue", "show", "OZX-4"), "联机死亡规则", "Epic memories")
	m.contains(m.run("memory", "search", "--issue", "OAE-1"), "epic OZX-1", "联机死亡规则")

	// Write-back that restates an existing decision is caught…
	m.contains(m.fails("memory", "add", "--epic", "OZX-3", "--title", "联机死亡规则（修订）",
		"--content", "联机死亡改为直接出局"), "seem to say the same thing", death, "mf memory update")
	// …and can be recorded deliberately.
	m.run("memory", "add", "--epic", "OZX-3", "--title", "联机死亡规则（修订）", "--content", "联机死亡改为直接出局", "--new")

	// 3. Promote a conclusion from issue A; issue B sees it, then sees it change.
	boss := m.memoryID(m.run("memory", "add", "--issue", "OZX-2", "--title", "Boss 血量公式", "--content", "hp = 100 * 1.15^wave"))
	m.contains(m.run("memory", "promote", boss), "moved memory to epic OZX-1: Boss 血量公式")
	ctx := m.run("--as", "bob", "issue", "context", "OZX-3")
	m.contains(ctx, "=== Epic OZX-1: 无尽模式", "无尽模式的总设计", "Boss 血量公式", "[NEW]", "hp = 100 * 1.15^wave",
		"OAE-1", "waits on OAE-1")
	m.omits(m.run("--as", "bob", "issue", "show", "OZX-3"), "*new", "*updated")
	m.run("memory", "update", boss, "--content", "hp = 120 * 1.2^wave")
	b := m.run("--as", "bob", "issue", "show", "OZX-3")
	m.contains(b, "Boss 血量公式", "*updated")
	// Read state is per reader: carol has read nothing.
	m.contains(m.run("--as", "carol", "issue", "show", "OZX-3"), "*new")
	// Moved, not copied: OZX-2 no longer has a memory of its own.
	m.omits(m.run("issue", "show", "OZX-2"), "\nMemories:")

	// 4. Taking an issue out drops it from the view, the numbers and the memories.
	m.contains(m.run("epic", "rm", "OAE-1"), "OAE-1: removed from OZX-1")
	show = m.run("epic", "show", "OZX-1")
	m.omits(show, "无尽模式图标导出")
	// OZX-3 still waits on OAE-1: the dependency outlives the membership.
	m.contains(show, "Progress: 1/2 done (50%)", "OZX-3 [todo] waits on OAE-1")
	m.omits(m.run("issue", "show", "OAE-1"), "Epic memories", "epic      OZX-1")
	m.contains(m.run("issue", "history", "OAE-1"), "epic")

	// Moving between epics in one step.
	m.run("epic", "create", "OZX", "--title", "联网 PvE")
	m.contains(m.run("epic", "add", "OZX-5", "OZX-3"), "OZX-3: moved from OZX-1 to OZX-5")

	// Rules: epics do not nest; an epic with issues keeps its type.
	m.contains(m.fails("epic", "add", "OZX-1", "OZX-5"), "epics do not nest")
	m.contains(m.fails("epic", "add", "OZX-4", "OZX-3"), "not an epic")
	m.contains(m.fails("issue", "update", "OZX-1", "--type", "requirement"), "still has 1 issue")
	m.contains(m.fails("issue", "update", "OZX-3", "--type", "epic"), "belongs to an epic")

	// Closing an epic needs no commit of its own.
	m.contains(m.run("epic", "close", "OZX-1"), "OZX-1: todo → in_progress → done")
}

// MF-39 acceptance 5: an issue outside any epic reads exactly as before.
func TestIntegrationEpicRegression(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("REG", "Regression")
	m.run("issue", "create", "REG", "--type", "bug", "--title", "plain bug")
	m.run("memory", "add", "--issue", "REG-1", "--title", "root cause", "--content", "the loader")
	show := m.run("issue", "show", "REG-1")
	m.contains(show, "Memories:", "root cause")
	m.omits(show, "epic", "Epic", "*new")
	j := m.json("issue", "show", "REG-1")
	if _, ok := j["epic_memories"]; ok {
		t.Errorf("epic_memories should not appear on an issue outside any epic: %v", j)
	}
	if _, ok := j["epic_id"]; ok {
		t.Errorf("epic_id should be omitted: %v", j)
	}
	m.contains(m.run("project", "progress", "REG"), "1 issue(s) total")
	m.contains(m.run("memory", "search", "--project", "REG"), "root cause")
}
