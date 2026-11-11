package mfcli

import "testing"

// MF-36: closing an issue as a duplicate of another, end to end.
func TestIntegrationDuplicate(t *testing.T) {
	m := newMFTest(t)
	m.seedProject("DUP", "Dedup")
	for _, title := range []string{"original", "filed again", "filed a third time", "the real root"} {
		m.run("issue", "create", "DUP", "--type", "bug", "--title", title)
	}

	m.contains(m.run("issue", "dup", "DUP-2", "DUP-1"), "DUP-2: todo → duplicate of DUP-1")
	m.contains(m.run("issue", "show", "DUP-2"), "status    duplicate", "dup of    DUP-1 [todo] original")
	a := m.run("issue", "show", "DUP-1")
	m.contains(a, "dups      DUP-2", "Comments: 1")
	m.contains(m.run("comment", "list", "DUP-1"), "DUP-2「filed again」已标记为本单的重复")

	// A target that is itself a duplicate resolves to the issue it points at.
	m.contains(m.run("issue", "dup", "DUP-3", "DUP-2"), "duplicate of DUP-1 (DUP-2 is itself a duplicate of it)")
	m.contains(m.run("issue", "show", "DUP-1"), "dups      DUP-2, DUP-3")
	// …so pointing DUP-1 at its own duplicate would be a self-reference.
	m.contains(m.fails("issue", "dup", "DUP-1", "DUP-3"), "cannot be a duplicate of itself")

	// Merging the target elsewhere carries its duplicates along.
	m.run("issue", "dup", "DUP-1", "DUP-4")
	m.contains(m.run("issue", "show", "DUP-4"), "dups      DUP-1, DUP-2, DUP-3")
	m.contains(m.run("issue", "show", "DUP-2"), "dup of    DUP-4")

	// Duplicates are closed: hidden by default, filterable on their own.
	open := m.run("issues", "DUP")
	m.contains(open, "DUP-4")
	m.omits(open, "DUP-2 ")
	m.contains(m.run("issues", "DUP", "--status", "duplicate"), "DUP-1", "DUP-2", "DUP-3")
	m.omits(m.run("issues", "DUP", "--status", "rejected"), "DUP-2")
	m.contains(m.run("project", "progress", "DUP"), "duplicate")

	// Entering "duplicate" needs a target.
	m.contains(m.fails("issue", "status", "DUP-4", "duplicate"), "mf issue dup DUP-4 <TARGET_KEY>")

	// Reopening drops the link; the history keeps it.
	m.run("issue", "status", "DUP-2", "todo")
	b := m.run("issue", "show", "DUP-2")
	m.contains(b, "status    todo")
	m.omits(b, "dup of")
	m.omits(m.run("issue", "show", "DUP-4"), "DUP-2,")
	m.contains(m.run("issue", "history", "DUP-2"), "duplicate_of")

	// Done issues are not duplicates; reopen first.
	m.run("issue", "done", "DUP-4", "--force")
	m.contains(m.fails("issue", "dup", "DUP-4", "DUP-2"), "cannot mark a done issue")

	// Duplicates stay out of the similar-issue pool.
	m.omits(m.run("issue", "similar", "DUP", "--title", "filed a third time"), "DUP-3")
}
