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

Memory Flow is driven entirely through the **`mf` CLI** — never `curl`. It resolves
the instance itself (remote home server, or the local standalone app) and enforces
the completion workflow. Run `mf help` or `mf help workflow` if you need the full
command list.

```
CI/CD API: https://cicd.local.playquota.com/api
```

---

## Step 1 — Identify the Issue

If the user specified an issue key (e.g. `MF-6`, `ORT-15`), fetch it directly:

```bash
mf issue show ISSUE-KEY
```

Otherwise, list open issues for the relevant project (`mf ctx` lists the projects):

```bash
mf issues PROJECT_KEY
```

Pick the highest-priority issue (P0 > P1 > P2). Check `mf issue show KEY --deps`
before starting: a `critical` dependency that isn't `done` blocks the work.

---

## Step 2 — Transition to In Progress

```bash
mf issue start ISSUE-KEY
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

```bash
mf issue create PROJECT_KEY --type bug --title "Short title" \
  --priority P1 --desc "What is broken and where"
```

Use `--type bug` for something broken, `--type requirement` for something missing.
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

## Step 5 — Record the Commit and Mark Done

One command records the commit URL and closes the issue:

```bash
mf issue done ISSUE-KEY --git HEAD
```

`mf` builds the commit URL from the project's `git_url` (falling back to the repo's
origin remote), walks any intermediate status hops, and **refuses to close an issue
with no `git_url`** — do not work around that with `--force`.

---

## Step 6 — CI/CD (optional, if user requests it)

Only trigger if the user explicitly asks for a build/deploy.

**Delegate entirely to the `cicd-manager` skill** — do not reimplement the CI/CD workflow here.
The cicd-manager skill handles: pre-flight git check, finding the app, triggering the build,
polling build status, deploying on success, and monitoring pods.

Invoke it with context:
- CI/CD platform URL: `https://cicd.local.playquota.com`
- App name: matches the project name in Memory Flow (e.g. `memory-flow`, `cicd-platform`)
- The commit has already been pushed at this point

The cicd-manager skill will:
1. Run pre-flight git check (verify clean + pushed)
2. Find the app by name
3. Trigger build with the current commit SHA
4. Poll build status until success or failure
5. Deploy the release
6. Confirm deployment status

---

## Rules

1. **Always record `git_url`** before marking an issue as `done` — `mf issue done --git HEAD` does both.
2. **Commit format** `[ISSUE-KEY] ...` is mandatory.
3. **One issue at a time** — complete and mark done before moving to the next.
4. **No git add .** — stage only changed files relevant to the issue.
5. **Suspend, don't fail silently** — if you cannot complete the issue, suspend it and explain why:

```bash
mf issue status ISSUE-KEY suspended
```

---

## Status Transition Reference

```
todo        → in_progress, suspended, rejected
in_progress → review, done, suspended, todo
review      → testing, in_progress
testing     → done, in_progress
done        → closed, in_progress
suspended   → todo
rejected    → todo
```

`mf issue status KEY TARGET` walks intermediate hops automatically (closing a
`todo` issue performs todo → in_progress → done).
