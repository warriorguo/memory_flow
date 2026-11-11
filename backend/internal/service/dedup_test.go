package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
)

func TestExtractIdentifiers(t *testing.T) {
	ids := extractIdentifiers("遥测会话 `2026-11-11_172018`（P2），房间 story_3_f1_room_4_-1 的 Right 门，" +
		"见 OZX-1956；版本 1.4.2，日期 2026-11-11，文件 Assets/Scripts/DoorSpawn.cs，`RoomEdge`")
	for _, want := range []string{"2026-11-11_172018", "story_3_f1_room_4_-1", "ozx-1956", "assets/scripts/doorspawn.cs", "roomedge"} {
		if !ids[want] {
			t.Errorf("missing identifier %q in %v", want, ids)
		}
	}
	for _, unwanted := range []string{"right", "1.4.2", "2026-11-11", "p2"} {
		if ids[unwanted] {
			t.Errorf("generic token %q should not be an identifier", unwanted)
		}
	}
}

func TestTitleBigramsJaccard(t *testing.T) {
	a := titleBigrams("进门落点压在坑沿，P2 坠落")
	if j := jaccard(a, titleBigrams("进门落点压在坑沿 P2坠落!")); j != 1 {
		t.Errorf("punctuation/space should not matter, got %v", j)
	}
	if j := jaccard(a, titleBigrams("Search returns empty results")); j != 0 {
		t.Errorf("unrelated titles should not overlap, got %v", j)
	}
}

// The motivating case (OZX-1956 / OZX-1958, paraphrased): two titles with
// almost no words in common, but the same session dir and room id.
func dedupPool() []model.Issue {
	now := time.Now()
	mk := func(key, title, desc string, age time.Duration) model.Issue {
		return model.Issue{ID: uuid.New(), IssueKey: key, Type: "bug", Status: "todo",
			Title: title, Description: strp(desc), CreatedAt: now.Add(-age)}
	}
	pool := []model.Issue{
		mk("GM-10", "联机：错开后的进门落点压在坑沿，P2 一进窄走廊房间就坠落",
			"会话 `2026-11-11_172018`，房间 story_3_f1_room_4_-1，Right 门，三次进门坠落。", 30*time.Minute),
		mk("GM-11", "chapter_4 开不了局：nursery 的死亡继承不在 floors[0]", "chapter_4 起局即报错。", 48*time.Hour),
		mk("GM-12", "Search returns empty results", "Keyword search returns nothing.", 72*time.Hour),
		mk("GM-13", "商店价格显示错误", "价格多显示一位小数。", 96*time.Hour),
	}
	// Many issues mention chapter_4, which makes it a generic identifier.
	for i := 0; i < 6; i++ {
		pool = append(pool, mk(fmt.Sprintf("GM-%d", 20+i), fmt.Sprintf("chapter_4 第 %d 个问题", i), "chapter_4 相关。", time.Duration(100+i)*time.Hour))
	}
	return pool
}

func TestRankSimilar_SharedIdentifiersBeatWording(t *testing.T) {
	draft := &model.Issue{Type: "bug",
		Title:       "story 房 story_3_f1_room_4_-1 进门落点在虚空格：进门 0.04s 即 fall_begin",
		Description: strp("会话目录 2026-11-11_172018，P2 从 Right 门进入后立即坠落；chapter_4。")}
	got := rankSimilar(draft, dedupPool(), "")
	if len(got) == 0 || got[0].Key != "GM-10" {
		t.Fatalf("expected GM-10 first, got %+v", got)
	}
	if got[0].Level != model.SimilarLikely {
		t.Errorf("two shared specific ids should be likely, got %s", got[0].Level)
	}
	if len(got[0].SharedIdentifiers) != 2 {
		t.Errorf("expected session dir + room id shared, got %v", got[0].SharedIdentifiers)
	}
	for _, s := range got {
		if s.Key != "GM-10" && s.Level == model.SimilarLikely {
			t.Errorf("%s shares only the generic chapter_4 and must not be likely", s.Key)
		}
	}
}

func TestRankSimilar_TitleAndReferenceAndExclude(t *testing.T) {
	pool := dedupPool()

	close := rankSimilar(&model.Issue{Title: "Search returns empty result"}, pool, "")
	if len(close) == 0 || close[0].Key != "GM-12" || close[0].Level != model.SimilarPossible {
		t.Errorf("near-identical title should be a possible match, got %+v", close)
	}

	ref := rankSimilar(&model.Issue{Title: "价格又错了", Description: strp("和 GM-13 是同一个现象")}, pool, "")
	if len(ref) == 0 || ref[0].Key != "GM-13" || !ref[0].References || ref[0].Level != model.SimilarPossible {
		t.Errorf("citing a key means related: a possible match, got %+v", ref)
	}

	none := rankSimilar(&model.Issue{Title: "完全无关的需求：导出 PDF"}, pool, "")
	if len(none) != 0 {
		t.Errorf("unrelated draft should match nothing, got %+v", none)
	}

	self := rankSimilar(&pool[0], pool, "GM-10")
	for _, s := range self {
		if s.Key == "GM-10" {
			t.Errorf("excluded key must not be returned")
		}
	}
}

func TestCreateWithSimilar(t *testing.T) {
	pool := dedupPool()
	newSvc := func(dedupErr error, history *[]string) *IssueService {
		issueRepo := &mocks.MockIssueRepo{
			BeginTxFn: func(ctx context.Context) (database.Tx, error) { return newMockTx(), nil },
			CreateFn: func(ctx context.Context, tx database.Tx, key string, pid uuid.UUID, req model.CreateIssueRequest) (*model.Issue, error) {
				return &model.Issue{ID: uuid.New(), IssueKey: key, Type: req.Type, Title: req.Title}, nil
			},
			ListDedupFn: func(ctx context.Context, pid uuid.UUID, since time.Time) ([]model.Issue, error) {
				if d := time.Since(since); d < 29*24*time.Hour || d > 31*24*time.Hour {
					t.Errorf("closed lookback should be 30 days, got %v", d)
				}
				return pool, dedupErr
			},
		}
		projectRepo := &mocks.MockProjectRepo{
			IncrementIssueNumberFn: func(ctx context.Context, tx database.Tx, id uuid.UUID) (int, string, error) { return 99, "GM", nil },
		}
		historyRepo := &mocks.MockIssueHistoryRepo{
			CreateFn: func(ctx context.Context, tx database.Tx, id uuid.UUID, field string, o, n *string, op *string) error {
				*history = append(*history, field+"="+*n)
				return nil
			},
		}
		return NewIssueService(issueRepo, projectRepo, historyRepo)
	}
	req := model.CreateIssueRequest{Type: "bug", Title: "Search returns empty result"}

	var history []string
	issue, similar, err := newSvc(nil, &history).CreateWithSimilar(context.Background(), uuid.New(), req)
	if err != nil || issue.IssueKey != "GM-99" {
		t.Fatalf("create failed: %v", err)
	}
	if len(similar) == 0 || similar[0].Key != "GM-12" {
		t.Errorf("expected GM-12 as similar, got %+v", similar)
	}
	if len(history) != 0 {
		t.Errorf("no judgement without --not-dup, got %v", history)
	}

	req.NotDuplicate = true
	if _, _, err := newSvc(nil, &history).CreateWithSimilar(context.Background(), uuid.New(), req); err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0] != "not_duplicate_of=GM-12" {
		t.Errorf("expected the judgement in history, got %v", history)
	}

	// A broken lookup must not stop the issue from being filed.
	issue, similar, err = newSvc(fmt.Errorf("db down"), &history).CreateWithSimilar(context.Background(), uuid.New(), req)
	if err != nil || issue == nil || similar != nil {
		t.Errorf("lookup failure should still create, got issue=%v similar=%v err=%v", issue, similar, err)
	}
}
