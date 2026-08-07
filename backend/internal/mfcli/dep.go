package mfcli

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

// cmdDepAdd links two issues. The API wants the target as a UUID, so the CLI
// accepts an issue key on both sides and resolves the target itself.
func cmdDepAdd(e *env, args []string) error {
	fs := newFlagSet("issue dep add")
	depType := fs.String("type", "depends_on", "depends_on (this issue needs the target) or blocks")
	severity := fs.String("severity", "critical", "critical (hard blocker) or recommended (soft association)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 2, "issue dep add <ISSUE_KEY> <TARGET_ISSUE_KEY> [--type depends_on|blocks] [--severity critical|recommended]"); err != nil {
		return err
	}

	target, _, err := e.fetchIssue(pos[1])
	if err != nil {
		return fmt.Errorf("resolve dependency target %s: %w", pos[1], err)
	}

	var dep model.IssueDependency
	raw, err := e.client.post("/api/v1/issues/"+pos[0]+"/dependencies", model.CreateDependencyRequest{
		TargetIssueID: target.ID,
		Type:          *depType,
		Severity:      *severity,
	}, &dep)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("%s %s %s (%s)\n", pos[0], *depType, target.IssueKey, *severity)
	return nil
}

func cmdDepList(e *env, args []string) error {
	fs := newFlagSet("issue dep list")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "issue dep list <ISSUE_KEY>"); err != nil {
		return err
	}

	deps, raw, err := e.fetchDeps(pos[0])
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	if len(deps) == 0 {
		e.printf("%s has no dependencies\n", pos[0])
		return nil
	}
	return e.printDeps(pos[0], deps)
}

func cmdDepRemove(e *env, args []string) error {
	fs := newFlagSet("issue dep rm")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 2, "issue dep rm <ISSUE_KEY> <DEPENDENCY_ID>"); err != nil {
		return err
	}
	raw, err := e.client.del("/api/v1/issues/" + pos[0] + "/dependencies/" + pos[1])
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("removed dependency %s from %s\n", pos[1], pos[0])
	return nil
}

func cmdDepTree(e *env, args []string) error {
	fs := newFlagSet("issue dep tree")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "issue dep tree <ISSUE_KEY>"); err != nil {
		return err
	}

	var root model.DependencyNode
	raw, err := e.client.get("/api/v1/issues/"+pos[0]+"/dependency-tree", &root)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	printDepTree(e.out, &root, "", "")
	return nil
}

func (e *env) fetchDeps(key string) ([]model.IssueDependency, []byte, error) {
	var deps []model.IssueDependency
	raw, err := e.client.get("/api/v1/issues/"+key+"/dependencies", &deps)
	if err != nil {
		return nil, nil, err
	}
	return deps, raw, nil
}

// printDeps renders dependencies from the perspective of the issue asked about.
// The list endpoint returns bare UUIDs in both directions, so the CLI resolves
// each counterpart issue and flips the relation when this issue is the target.
func (e *env) printDeps(key string, deps []model.IssueDependency) error {
	self, _, err := e.fetchIssue(key)
	if err != nil {
		return err
	}

	resolved := map[uuid.UUID]*model.Issue{self.ID: self}
	rows := make([][]string, 0, len(deps))
	for _, dep := range deps {
		otherID, relation := dep.TargetIssueID, dep.Type
		if dep.SourceIssueID != self.ID {
			otherID = dep.SourceIssueID
			relation = inverseRelation(dep.Type)
		}

		other, ok := resolved[otherID]
		if !ok {
			other, _, err = e.fetchIssue(otherID.String())
			if err != nil {
				return fmt.Errorf("resolve dependency issue %s: %w", otherID, err)
			}
			resolved[otherID] = other
		}

		rows = append(rows, []string{
			relation,
			other.IssueKey,
			dep.Severity,
			other.Status,
			truncate(other.Title, 50),
			dep.ID.String(),
		})
	}
	table(e.out, []string{"RELATION", "ISSUE", "SEVERITY", "STATUS", "TITLE", "DEP_ID"}, rows)
	return nil
}

// inverseRelation restates a dependency from the target's point of view: if
// they depend on us we block them, and if they block us we are blocked by them.
func inverseRelation(depType string) string {
	if depType == "depends_on" {
		return "blocks"
	}
	return "blocked_by"
}
