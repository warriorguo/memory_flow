---
name: memory-flow-pm
description: >
  Interact with the Memory Flow project management platform.
  Use this skill when: (1) the user asks about current bugs, requirements, or issue status;
  (2) the user wants to create/file a bug or requirement;
  (3) the user wants to record or retrieve a memory (recall/write);
  (4) the user asks about project progress or status;
  (5) the user wants to create or manage a project;
  (6) the user wants to attach, fetch, replace, or delete a file (asset) on an issue.
  Trigger on phrases like "file a bug", "create a requirement", "what are the open issues",
  "record this", "recall memory", "what's the project status", "list bugs", "check progress",
  "create project", "update issue", "mark as done", "attach a file", "upload the screenshot",
  "get the assets for this issue", "replace the reference art".
compatibility: Uses the `mf` CLI, which reaches the remote Memory Flow API or falls back to the local standalone app
allowed-tools: Bash(mf:*)
metadata:
  author: warriorguo
  version: "6.0"
  service-url: "https://memory-flow.local.playquota.com"
  local-fallback-url: "http://127.0.0.1:8080"
---

# Memory Flow Project Management Skill

Manage projects, issues (bugs/requirements), progress, and memories on the Memory Flow platform.

Everything goes through the **`mf` CLI**. Do not use `curl`, `jq`, or `python3`
against the API — `mf` covers every endpoint, resolves the instance itself, and
formats output for reading. If a command you need seems to be missing, check
`mf help <topic>` before reaching for `curl`.

## Activation (run FIRST, every time)

```bash
mf ctx
```

This prints the instance in use and every project's **key**, **name**, **status**,
and **summary**. Internalize them so you can:
- Route issues to the correct project when filing bugs/requirements
- Infer which project the user means from context (a frontend bug belongs to the project with a frontend scope)
- Avoid asking which project to use when it is obvious

If `mf ctx` reports a **local** instance, tell the user — the local standalone's
data may lag the home server until synced.

> **Instance resolution** is automatic: `--url` / `$MEMORY_FLOW_URL`, else the
> remote home server, else the local standalone app (`~/.memory_flow/endpoint`,
> then `http://127.0.0.1:8080`). `mf endpoint` shows which one is live. If
> everything is unreachable, `mf` says so and exits non-zero — relay that and
> suggest starting the local app (`open "/Applications/Memory Flow.app"`).

## Global flags

Accepted anywhere in the command line:

| Flag | Effect |
|------|--------|
| `--json` | Print the raw API response instead of formatted text |
| `--url <URL>` | Pin the instance instead of auto-resolving |
| `--refresh` | Ignore the cached endpoint and probe again |
| `--timeout <SECONDS>` | Request timeout (default 30) |

Default to the formatted output. Reach for `--json` only when you need a field
the text view omits.

---

## Issues (bugs / requirements)

### List

```bash
mf issues MF                          # open issues only (the usual question)
mf issues MF --priority P0
mf issues MF --type bug --assignee andrew
mf issues MF --keyword "atlas export"
mf issues MF --all                    # include done/closed/rejected
```

Filters: `--status`, `--type` (bug/requirement), `--priority` (P0/P1/P2),
`--assignee`, `--keyword`, `--all`, `--limit`, `--page`.

Output is already the table to present: Key | Title | Type | Priority | Status | Assignee.

### Show

```bash
mf issue show ORT-100
mf issue show ORT-100 --deps --history
```

### Filing issues (analyze-then-create workflow)

When a user describes a bug or requirement, **before creating anything**:

**Step 1 — Analyze scope.** Does it span multiple subsystems? Are there
sequential steps with dependencies? Does it mix a bug fix with a new feature?
Or is it a single, well-scoped change? If it's single and well-scoped, skip to
Step 3.

**Step 2 — Propose a decomposition** and wait for the user to confirm, adjust,
or override:

> **Proposed issue breakdown:**
>
> 1. `[requirement]` P1 — Title of first issue
> 2. `[requirement]` P2 — Title of second issue
> 3. `[bug]` P1 — Title of third issue
>
> **Dependencies:**
> - #2 depends on #1 (critical) — cannot start without #1's API
> - #3 depends on #1 (recommended) — related but not blocking

**Step 3 — Create**, in dependency order (dependencies first):

```bash
mf issue create MF --type bug --title "Search returns empty results" \
  --priority P1 --assignee andrew --desc "Detailed description"
```

For a long or multi-line description, pipe it in rather than quoting it:

```bash
mf issue create MF --type requirement --title "Add export button" --desc-file - <<'EOF'
Steps to reproduce…

Expected: …
EOF
```

Required: `--type` (bug/requirement), `--title`.
Optional: `--desc`/`--desc-file`, `--priority` (default P2), `--assignee`,
`--source`, `--version`, `--git-url`, `--pr-url`, `--doc-url`.

Priority: **P0** blocking, fix immediately · **P1** important, not blocking the
core flow · **P2** normal, schedulable.

**Step 4 — Set dependencies** (see below), then report a summary table:

> | Key | Title | Type | Priority | Depends On |
> |-----|-------|------|----------|------------|
> | MF-9 | Backend API for X | requirement | P1 | — |
> | MF-10 | Frontend for X | requirement | P2 | MF-9 (critical) |

### Update

```bash
mf issue update MF-1 --title "Updated title" --priority P0 --assignee someone
mf issue update MF-1 --desc-file notes.md
```

Updatable: `--title`, `--desc`/`--desc-file`, `--priority`, `--assignee`,
`--type`, `--source`, `--version`, `--git-url`, `--pr-url`, `--doc-url`.
All changes are recorded in the issue history automatically.

### Status

```bash
mf issue start MF-1              # → in_progress
mf issue status MF-1 review
mf issue status MF-1 done        # walks intermediate hops as needed
```

The workflow graph:

```
todo        → in_progress, suspended, rejected
in_progress → review, done, suspended, todo
review      → testing, in_progress
testing     → done, in_progress
done        → closed, in_progress
suspended   → todo
rejected    → todo
```

`mf` walks multi-hop paths for you — closing a `todo` issue performs
todo → in_progress → done. Pass `--direct` to require a single legal hop.

### History

```bash
mf issue history MF-1
```

---

## Completing an issue (required workflow)

An issue must carry its commit link before it closes. One command does both:

```bash
mf issue done MF-1 --git HEAD
```

`--git` accepts a full URL, a short sha, or any commit-ish (`HEAD`, a branch).
The commit URL is built from the project's `git_url`, falling back to the origin
remote of `--repo` (default: the current directory). Split into two steps when
you prefer:

```bash
mf issue attach-git MF-1 5253083     # or: --pr 42 to also record the PR
mf issue done MF-1
```

`mf issue done` **refuses** to close an issue whose `git_url` is empty. That
guard is the point — do not reach for `--force` unless the user asks for it.

**Commit message format** — every commit for an issue:

```
[{ISSUE_KEY}] description of the change
```

Examples: `[MF-1] Add image rotation support` · `[MF-3] Fix memory search returning empty results`

---

## Dependencies

Dependencies work across projects (ORT-20 can depend on MF-5).

```bash
mf issue dep add MF-10 MF-9 --type depends_on --severity critical
mf issue dep list MF-10
mf issue dep tree MF-10
mf issue dep rm MF-10 <DEPENDENCY_ID>
mf issue priority MF-10          # effective priority, including inherited
```

- `--type`: `depends_on` (this issue needs the target) or `blocks` (this issue blocks the target)
- `--severity`: `critical` (hard blocker — target must finish first; priority inherits upward) or `recommended` (soft association, no inheritance)

Both sides are given as issue keys; `mf` resolves the UUIDs the API wants.

---

## Projects

```bash
mf projects                            # or: mf projects --status active
mf project show MF
mf project create NEW --name "New Project" --summary "…" --git-url "https://github.com/…"
mf project update MF --name "New Name" --status active
mf project archive MF
```

Project key: uppercase alphanumeric, 2–10 chars. Required on create: `<KEY>` and `--name`.
Optional: `--summary`, `--desc`/`--desc-file`, `--design-principles`, `--git-url`,
`--cicd-url`, `--doc-url`, `--owner`.

`mf project update` also takes `--next-issue-number N` to bump the issue-key counter forward.

---

## Progress

```bash
mf project progress MF
mf project progress MF --trend 30
```

**Summarize in natural language**, e.g.:
> Project MF: 20 issues total — 3 todo, 1 in progress, 16 done. 10 P1, 10 P2.

---

## Memories

```bash
mf memory add --title "Why sync uses last-write-wins" --content "…" --project MF
mf memory add --title "Root cause of MF-11" --content-file notes.md --issue MF-11
mf memory search "sync conflict" --project MF
mf memory search --project MF --type recall --full
mf memory show <MEMORY_ID>
mf memory update <MEMORY_ID> --content "…"
mf memory rm <MEMORY_ID>
```

- **recall** — reusable project context: design decisions, root causes, constraints, decision records
- **write** — produced artifacts: drafts, task summaries, supplementary context

`--issue` attaches the memory to an issue and infers its project.

---

## Assets (files attached to an issue)

Every issue can carry files — reference art, a screen recording, a crash log,
sample data, a design doc. They are addressed by **filename, unique per issue**,
and the description can point at one with `asset:<filename>`.

```bash
mf asset add OZX-12 ~/art/enemy_ref.png ~/audio/hit.wav   # one or many files
mf asset add OZX-12 --file - --name report.md             # content from stdin
mf asset list OZX-12
mf asset get OZX-12 enemy_ref.png -o ./enemy_ref.png      # '-o -' writes stdout
mf asset get OZX-12 --all -o ./assets                     # every file at once
mf asset replace OZX-12 enemy_ref.png ~/art/enemy_v2.png
mf asset rm OZX-12 old.png --yes
```

`mf issue show <KEY>` lists the issue's assets, so you learn the material exists
without a second command.

**Uploading over an existing filename fails** with a conflict rather than
overwriting — pass `--overwrite`, or use `replace`, when replacing is what you
mean. Replacing keeps the name, so `asset:` references in the description stay
valid. Single files are capped (32MB by default); the server's error says so.

### Asset or memory?

- **asset** — a *file* that is an input to, or an output of, the work: images,
  audio, video, screen recordings, logs, sample saves, design docs, generated
  atlases.
- **memory** — reusable *knowledge*: a design decision, a root cause, a
  constraint. Text that a future reader needs to understand the project, not a
  file they need to open.

A crash log is an asset. The conclusion you drew from reading it is a memory.

### Referencing assets from a description

```
参考图 ![参考](asset:enemy_ref.png)，音效见 asset:hit.wav
```

References resolve within the issue that owns the description. The web UI
renders images inline and streams video/audio; `mf issue show` marks each
reference `(asset)` or `⚠ missing asset`. Deleting a referenced file is allowed
but warns.

### Usage scenarios (the OZX closed loop)

The point of assets is that one issue can carry a task from proposal through
material handoff to landed implementation.

1. **Filing a requirement with material** — a designer asks for a new enemy.
   Attach the reference art, the sound effect, and the animation table to the
   issue, and reference them in the description:

   ```bash
   mf issue create OZX --type requirement --title "新增敌人：爆裂虫" --desc-file - <<'EOF'
   外观参考 asset:enemy_ref.png，受击音效 asset:hit.wav。
   动画帧表见 asset:frames.csv。
   EOF
   mf asset add OZX-42 ~/art/enemy_ref.png ~/audio/hit.wav ~/data/frames.csv
   ```

2. **Picking the issue up** — pull the material into the working directory
   *before* touching code, so the implementation works from the real files:

   ```bash
   mf asset get OZX-42 --all -o ./.work/OZX-42
   ```

3. **Handing back the result** — attach the proof of work to the same issue, so
   review does not require rebuilding anything:

   ```bash
   mf asset add OZX-42 ./screenshots/in_game.png ./recordings/attack.mp4
   ```

4. **The source file is revised** — art redraws the reference. Replace it under
   the same name and every existing reference keeps working:

   ```bash
   mf asset replace OZX-42 enemy_ref.png ~/art/enemy_v2.png
   ```

5. **Reporting a bug with evidence** — attach the repro recording, the crash
   log, and the save file that triggers it:

   ```bash
   mf issue create OZX --type bug --title "第三关 Boss 卡墙" --priority P1 \
     --desc "复现录屏 asset:repro.mp4，日志 asset:crash.log，存档 asset:save.dat"
   mf asset add OZX-43 ./repro.mp4 ./crash.log ./save.dat
   ```

---

## Tags

```bash
mf tags
mf tag create frontend --color '#1890ff'
mf issue tag MF-1 frontend backend      # creates any tag that doesn't exist yet
mf issue untag MF-1 frontend
```

---

## Command reference

| Action | Command |
|--------|---------|
| Bootstrap (endpoint + projects) | `mf ctx` |
| Which instance am I on | `mf endpoint` |
| List projects | `mf projects` |
| Show / create / update / archive project | `mf project show\|create\|update\|archive …` |
| Project progress | `mf project progress <KEY> [--trend N]` |
| List issues | `mf issues <PROJECT_KEY> [filters]` |
| Show issue | `mf issue show <KEY> [--deps] [--history]` |
| Create issue | `mf issue create <PROJECT_KEY> --type T --title "…"` |
| Update issue | `mf issue update <KEY> [--field …]` |
| Start / transition | `mf issue start <KEY>` · `mf issue status <KEY> <STATUS>` |
| Record commit | `mf issue attach-git <KEY> [<sha\|url>]` |
| Complete issue | `mf issue done <KEY> --git HEAD` |
| Issue history | `mf issue history <KEY>` |
| Dependencies | `mf issue dep add\|list\|tree\|rm …` |
| Effective priority | `mf issue priority <KEY>` |
| Assets | `mf asset add\|list\|get\|replace\|rm <ISSUE_KEY> …` |
| Pull every asset | `mf asset get <KEY> --all -o <DIR>` |
| Memories | `mf memory add\|search\|show\|update\|rm …` |
| Tags | `mf tags` · `mf tag create` · `mf issue tag\|untag` |
| Help | `mf help [issue\|project\|asset\|memory\|tag\|workflow]` |

Every command accepts an issue key (`MF-1`) or project key (`MF`) wherever an
identifier is expected; UUIDs also work.

---

## Tips

1. **Infer type from context**: something broken = `bug`; something new = `requirement`
2. **Choose memory type wisely**: `recall` for reusable context, `write` for output artifacts
3. **Files go in assets, knowledge goes in memories** — attach the crash log as an asset, record what it revealed as a memory
4. **Issue keys** are auto-generated as `{PROJECT_KEY}-{N}` (e.g. MF-1, MF-2)
5. **Open items are the default** — `mf issues <KEY>` already hides done/closed/rejected; add `--all` when the user asks for everything
6. **Summarize progress in prose**, don't dump the table
7. **Analyze before filing**: decide one issue vs. several *before* creating anything; present the decomposition and wait for confirmation
8. **Set dependencies after batch creation**: `critical` for hard blockers, `recommended` for soft associations
9. **Completing issues**: `mf issue done <KEY> --git HEAD`, with `[ISSUE_KEY] description` in the commit message
10. **No auth needed** — all endpoints are public
11. **If `mf` is missing**, build and install it from the memory_flow repo: `make install-mf PREFIX=/opt/homebrew`
