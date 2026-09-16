package synccore_test

import (
	"context"
	"testing"

	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/synccore"
)

func author(s string) *string { return &s }

// A thread and its read receipts have to survive a sync, or the same comments
// come back unread on the other instance.
func TestCommentsAndReceiptsSync(t *testing.T) {
	ctx := context.Background()
	local := newDB(t, "local.db")
	server := newDB(t, "server.db")

	issueID := seedIssue(t, local, "MF")
	localComments := repository.NewCommentRepo(local)

	if _, err := localComments.Create(ctx, issueID, model.CreateCommentRequest{
		Body: "Repro'd on 1.4.2", AuthorID: author("alice"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := localComments.MarkRead(ctx, issueID, "bob"); err != nil {
		t.Fatal(err)
	}
	unreadForBob, err := localComments.Create(ctx, issueID, model.CreateCommentRequest{
		Body: "And on 1.4.3", AuthorID: author("alice"),
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := synccore.Export(ctx, local)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Comments) != 2 {
		t.Fatalf("snapshot carries %d comments, want 2", len(snap.Comments))
	}
	if len(snap.CommentReads) != 1 {
		t.Fatalf("snapshot carries %d receipts, want 1", len(snap.CommentReads))
	}
	if _, err := synccore.Import(ctx, server, snap); err != nil {
		t.Fatal(err)
	}

	serverComments := repository.NewCommentRepo(server)
	got, err := serverComments.ListByIssueID(ctx, issueID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("server has %d comments, want 2", len(got))
	}

	// Bob's receipt travelled with the thread, so the comment he had already read
	// does not come back as new — and the one he had not still does.
	read, err := serverComments.ReadIDs(ctx, issueID, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != 1 {
		t.Fatalf("server has %d receipts for bob, want 1", len(read))
	}
	if read[unreadForBob.ID] {
		t.Errorf("the comment bob had not read came back marked read")
	}

	// A second import is a no-op rather than a duplicate.
	if _, err := synccore.Import(ctx, server, snap); err != nil {
		t.Fatal(err)
	}
	if got, _ := serverComments.ListByIssueID(ctx, issueID); len(got) != 2 {
		t.Errorf("re-import produced %d comments, want 2", len(got))
	}
	if read, _ := serverComments.ReadIDs(ctx, issueID, "bob"); len(read) != 1 {
		t.Errorf("re-import produced %d receipts, want 1", len(read))
	}
}

// A comment whose issue never arrived is skipped and reported, not silently
// dropped — the same contract the rest of the merge follows.
func TestCommentSyncSkipsCommentWithoutItsIssue(t *testing.T) {
	ctx := context.Background()
	local := newDB(t, "local.db")
	server := newDB(t, "server.db")

	issueID := seedIssue(t, local, "MF")
	if _, err := repository.NewCommentRepo(local).Create(ctx, issueID,
		model.CreateCommentRequest{Body: "orphan"}); err != nil {
		t.Fatal(err)
	}

	snap, err := synccore.Export(ctx, local)
	if err != nil {
		t.Fatal(err)
	}
	snap.Projects = nil
	snap.Issues = nil

	res, err := synccore.Import(ctx, server, snap)
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped == 0 || len(res.Errors) == 0 {
		t.Errorf("import = %+v, want the orphaned comment skipped and reported", res)
	}
	if got, _ := repository.NewCommentRepo(server).ListByIssueID(ctx, issueID); len(got) != 0 {
		t.Errorf("server has %d comments, want 0", len(got))
	}
}
