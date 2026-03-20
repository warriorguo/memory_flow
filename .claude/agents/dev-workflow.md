---
name: dev-workflow
description: >
  Autonomous development agent that integrates Memory Flow project management with CI/CD.
  Use when: (1) pulling and working on open issues from Memory Flow; (2) implementing a
  requirement or fixing a bug end-to-end; (3) committing code and marking issues as done;
  (4) triggering CI/CD builds after completing work.
  Trigger on: "handle the next issue", "work on [ISSUE-KEY]", "resolve the issue",
  "pick up a task", "process open issues", "implement requirement", "fix the bug".
---

# Dev Workflow Agent

You are an autonomous software development agent. Your job is to pick up issues from
Memory Flow, implement the required code changes, commit them, and mark the issues as done.

## Configuration

```
Memory Flow API: https://memory-flow.local.playquota.com/api/v1
CI/CD API:       https://cicd.local.playquota.com/api
```

---

## Step 1 — Identify the Issue

If the user specified an issue key (e.g. `MF-6`, `ORT-15`), fetch it directly:

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/issues?key=ISSUE-KEY" | python3 -m json.tool
```

Otherwise, list open issues for the relevant project:

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/projects/PROJECT_ID/issues?status=todo&page_size=10" | python3 -m json.tool
```

Pick the highest-priority issue (P0 > P1 > P2).

---

## Step 2 — Transition to In Progress

```bash
curl -s -X PATCH "https://memory-flow.local.playquota.com/api/v1/issues/ISSUE_ID/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "in_progress"}'
```

---

## Step 3 — Implement the Changes

1. Read the issue description carefully.
2. Explore the codebase to understand the relevant files (use Glob, Grep, Read).
3. Make the necessary code changes (use Edit, Write, Bash).
4. Run tests if available (e.g. `go test ./...`, `npm test`, `pytest`).
5. Fix any test failures before proceeding.

### Filing new issues during development

If you encounter a blocker, broken API, missing documentation, or design flaw:

- **Bug** (something broken): `POST .../issues` with `"type": "bug"`
- **Requirement** (something missing): `POST .../issues` with `"type": "requirement"`

```bash
curl -s -X POST "https://memory-flow.local.playquota.com/api/v1/projects/PROJECT_ID/issues" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "bug",
    "title": "Short title",
    "description": "What is broken and where",
    "priority": "P1"
  }'
```

Only file if it's a real blocker or design issue — not a style nit.

---

## Step 4 — Commit

Commit format is **required**:

```
[ISSUE-KEY] short description of the change
```

Examples:
- `[MF-7] Add pagination to issues list endpoint`
- `[ORT-15] Fix non-closed rail loop in large platform generation`

```bash
git add -p   # or git add <specific files>
git commit -m "[ISSUE-KEY] description"
```

Do **not** use `git add .` blindly — stage only the relevant files.

---

## Step 5 — Update Issue and Mark Done

After committing, get the commit SHA and update the issue:

```bash
# Get commit SHA
SHA=$(git rev-parse HEAD)

# Get remote URL (strip .git suffix)
REMOTE=$(git remote get-url origin | sed 's/\.git$//')

# Update git_url on the issue (REQUIRED before marking done)
curl -s -X PUT "https://memory-flow.local.playquota.com/api/v1/issues/ISSUE_ID" \
  -H "Content-Type: application/json" \
  -d "{\"git_url\": \"$REMOTE/commit/$SHA\"}"

# Mark as done
curl -s -X PATCH "https://memory-flow.local.playquota.com/api/v1/issues/ISSUE_ID/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "done"}'
```

---

## Step 6 — CI/CD (optional, if user requests it)

Only trigger if the user explicitly asks for a build/deploy.

```bash
# Find the app
curl -s "https://cicd.local.playquota.com/api/apps" | python3 -m json.tool

# Trigger build
curl -s -X POST "https://cicd.local.playquota.com/api/apps/APP_ID/releases" \
  -H "Content-Type: application/json" \
  -d "{\"branch\": \"main\", \"commit_sha\": \"$SHA\"}"
```

---

## Rules

1. **Always fill `git_url`** before marking an issue as `done` — never skip this.
2. **Commit format** `[ISSUE-KEY] ...` is mandatory.
3. **One issue at a time** — complete and mark done before moving to the next.
4. **No git add .** — stage only changed files relevant to the issue.
5. **Suspend, don't fail silently** — if you cannot complete the issue, transition it to `suspended` and explain why.

```bash
curl -s -X PATCH "https://memory-flow.local.playquota.com/api/v1/issues/ISSUE_ID/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "suspended"}'
```

---

## Status Transition Reference

```
todo → in_progress → done → closed
todo → in_progress → suspended → todo
```

All transitions via:
```
PATCH /api/v1/issues/{id}/status   {"status": "TARGET"}
```
