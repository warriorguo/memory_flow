---
name: memory-flow-pm
description: >
  Interact with the Memory Flow project management platform.
  Use this skill when: (1) the user asks about current bugs, requirements, or issue status;
  (2) the user wants to create/file a bug or requirement;
  (3) the user wants to record or retrieve a memory (recall/write);
  (4) the user asks about project progress or status;
  (5) the user wants to create or manage a project.
  Trigger on phrases like "file a bug", "create a requirement", "what are the open issues",
  "record this", "recall memory", "what's the project status", "list bugs", "check progress",
  "create project", "update issue", "mark as done".
compatibility: Requires network access to the Memory Flow API
allowed-tools: Bash(curl:*)
metadata:
  author: warriorguo
  version: "2.0"
  service-url: "https://memory-flow.local.playquota.com"
---

# Memory Flow Project Management Skill

Interact with the Memory Flow project management platform to manage projects, issues (bugs/requirements), track progress, and manage memories.

## Configuration

```
Base URL: https://memory-flow.local.playquota.com
API Prefix: /api/v1
```

No authentication required. All API endpoints are public.

---

## Project Management

### List Projects

```bash
curl -s https://memory-flow.local.playquota.com/api/v1/projects | python3 -m json.tool
```

Supports query params: `name`, `status` (active/paused/archived), `owner_id`, `page`, `page_size`.

### Create Project

```bash
curl -s -X POST https://memory-flow.local.playquota.com/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "key": "MF",
    "name": "Memory Flow",
    "summary": "Project management platform",
    "git_url": "https://github.com/warriorguo/memory_flow.git",
    "owner_id": "admin"
  }' | python3 -m json.tool
```

Required: `key` (uppercase alphanumeric, 2-10 chars), `name`.
Optional: `summary`, `description`, `design_principles`, `git_url`, `cicd_url`, `doc_url`, `owner_id`.

### Get / Update / Archive Project

```bash
# Get
curl -s https://memory-flow.local.playquota.com/api/v1/projects/{id} | python3 -m json.tool

# Update
curl -s -X PUT https://memory-flow.local.playquota.com/api/v1/projects/{id} \
  -H "Content-Type: application/json" \
  -d '{"name": "New Name", "status": "active"}' | python3 -m json.tool

# Archive
curl -s -X DELETE https://memory-flow.local.playquota.com/api/v1/projects/{id} | python3 -m json.tool
```

---

## Issue Management (Bugs / Requirements)

### List Issues

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/projects/{projectId}/issues?page=1&page_size=20" | python3 -m json.tool
```

Query params (all optional):
- `type`: `requirement` or `bug`
- `status`: `todo`, `in_progress`, `review`, `testing`, `done`, `closed`, `rejected`
- `priority`: `P0`, `P1`, `P2`
- `assignee_id`, `keyword`, `page`, `page_size`

**Present results as a clean table:** Key | Title | Type | Priority | Status | Assignee

### Create Issue

```bash
curl -s -X POST "https://memory-flow.local.playquota.com/api/v1/projects/{projectId}/issues" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "bug",
    "title": "Issue title",
    "description": "Detailed description",
    "priority": "P1",
    "assignee_id": "someone"
  }' | python3 -m json.tool
```

Required: `type` (bug/requirement), `title`.
Optional: `description`, `priority` (P0/P1/P2, default P2), `assignee_id`, `source`, `version`, `git_url`, `pr_url`, `doc_url`.

Priority guidelines:
- **P0**: Blocking, must fix immediately
- **P1**: Important but not blocking core flow
- **P2**: Normal, can be scheduled

After creation, confirm with the issue key (e.g., "MF-3").

### Get Issue Detail

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}" | python3 -m json.tool
```

### Update Issue

```bash
curl -s -X PUT "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Updated title",
    "priority": "P0",
    "assignee_id": "new-assignee"
  }' | python3 -m json.tool
```

Updatable fields: `title`, `description`, `priority`, `assignee_id`, `source`, `version`, `git_url`, `pr_url`, `doc_url`.
All field changes are automatically tracked in issue history.

### Transition Issue Status

```bash
curl -s -X PATCH "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "in_progress"}' | python3 -m json.tool
```

Allowed transitions:
```
todo        -> in_progress, rejected
in_progress -> review, done, todo
review      -> testing, in_progress
testing     -> done, in_progress
done        -> closed, in_progress
rejected    -> todo
```

### Get Issue History

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}/history" | python3 -m json.tool
```

---

## Memory Management

### Create Memory

```bash
curl -s -X POST "https://memory-flow.local.playquota.com/api/v1/memories" \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "{projectId}",
    "type": "recall",
    "title": "Memory title",
    "content": "Detailed content to remember",
    "source_object_type": "project",
    "source_object_id": "{objectId}"
  }' | python3 -m json.tool
```

Required: `type` (recall/write), `title`, `content`.
Optional: `project_id`, `source_object_type` (project/requirement/bug), `source_object_id`.

Memory types:
- **recall**: Searchable project context — design decisions, root causes, constraints, decision records
- **write**: Written artifacts — AI-generated drafts, task summaries, supplementary context

### List / Search Memories

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/memories?project_id={projectId}&type={type}&keyword={keyword}&page=1&page_size=20" | python3 -m json.tool
```

Query params: `project_id`, `type` (recall/write), `keyword`, `page`, `page_size`.

### Get / Update / Delete Memory

```bash
# Get
curl -s "https://memory-flow.local.playquota.com/api/v1/memories/{id}" | python3 -m json.tool

# Update
curl -s -X PUT "https://memory-flow.local.playquota.com/api/v1/memories/{id}" \
  -H "Content-Type: application/json" \
  -d '{"title": "Updated", "content": "New content"}' | python3 -m json.tool

# Delete
curl -s -X DELETE "https://memory-flow.local.playquota.com/api/v1/memories/{id}"
```

---

## Progress & Statistics

### Progress Summary

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/projects/{projectId}/progress/summary" | python3 -m json.tool
```

Returns: `status_counts` (map), `priority_counts` (map), `type_counts` (map), `total`.

**Present as natural language**, e.g.:
> Project MF: 12 total issues (3 done, 5 in progress, 4 todo). 1 P0, 3 P1, 8 P2.

### Trend Data

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/projects/{projectId}/progress/trend?days=30" | python3 -m json.tool
```

Returns daily `created` and `done` counts.

---

## Tags

```bash
# List tags
curl -s https://memory-flow.local.playquota.com/api/v1/tags | python3 -m json.tool

# Create tag
curl -s -X POST https://memory-flow.local.playquota.com/api/v1/tags \
  -H "Content-Type: application/json" \
  -d '{"name": "frontend", "color": "#1890ff"}' | python3 -m json.tool

# Add tag to issue
curl -s -X POST "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}/tags" \
  -H "Content-Type: application/json" \
  -d '{"tag_id": "{tagId}"}' | python3 -m json.tool

# Remove tag from issue
curl -s -X DELETE "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}/tags/{tagId}"

# Add/remove tag to/from memory (same pattern)
curl -s -X POST "https://memory-flow.local.playquota.com/api/v1/memories/{memoryId}/tags" \
  -H "Content-Type: application/json" \
  -d '{"tag_id": "{tagId}"}' | python3 -m json.tool
```

---

## API Quick Reference

| Action | Method | Endpoint |
|--------|--------|----------|
| List projects | GET | `/api/v1/projects` |
| Create project | POST | `/api/v1/projects` |
| Get project | GET | `/api/v1/projects/{id}` |
| Update project | PUT | `/api/v1/projects/{id}` |
| Archive project | DELETE | `/api/v1/projects/{id}` |
| List issues | GET | `/api/v1/projects/{projectId}/issues` |
| Create issue | POST | `/api/v1/projects/{projectId}/issues` |
| Get issue | GET | `/api/v1/issues/{id}` |
| Update issue | PUT | `/api/v1/issues/{id}` |
| Transition status | PATCH | `/api/v1/issues/{id}/status` |
| Issue history | GET | `/api/v1/issues/{id}/history` |
| Progress summary | GET | `/api/v1/projects/{id}/progress/summary` |
| Progress trend | GET | `/api/v1/projects/{id}/progress/trend` |
| List memories | GET | `/api/v1/memories` |
| Create memory | POST | `/api/v1/memories` |
| Get memory | GET | `/api/v1/memories/{id}` |
| Update memory | PUT | `/api/v1/memories/{id}` |
| Delete memory | DELETE | `/api/v1/memories/{id}` |
| List tags | GET | `/api/v1/tags` |
| Create tag | POST | `/api/v1/tags` |
| Add tag to issue | POST | `/api/v1/issues/{id}/tags` |
| Remove tag from issue | DELETE | `/api/v1/issues/{id}/tags/{tagId}` |
| Add tag to memory | POST | `/api/v1/memories/{id}/tags` |
| Remove tag from memory | DELETE | `/api/v1/memories/{id}/tags/{tagId}` |

## Response Format

List: `{"data": [...], "total": N, "page": N, "page_size": N}`
Single: `{"data": {...}}`
Error: `{"error": "message"}`

---

## Completing an Issue (Required Workflow)

When an issue is done, you **MUST** follow these steps before marking it as `done`:

1. **Fill `git_url`** — update the issue with the commit URL or PR link. This field must NOT be left empty.

```bash
curl -s -X PUT "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}" \
  -H "Content-Type: application/json" \
  -d '{"git_url": "https://github.com/owner/repo/commit/{sha}"}' | python3 -m json.tool
```

2. **Commit message format** — all commits related to an issue must follow this format:

```
[{ISSUE_KEY}] description of the change
```

Examples:
- `[MF-1] Add image rotation support`
- `[MF-3] Fix memory search returning empty results`

3. **Transition to done** — only after `git_url` is set.

```bash
curl -s -X PATCH "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "done"}' | python3 -m json.tool
```

---

## Tips

1. **Infer type from context**: something broken = `bug`; something new = `requirement`
2. **Choose memory type wisely**: `recall` for reusable context, `write` for output artifacts
3. **Project key format**: uppercase alphanumeric, 2-10 chars (e.g., MF, PROJ)
4. **Issue keys** are auto-generated as `{PROJECT_KEY}-{N}` (e.g., MF-1, MF-2)
5. **Default to open items** when listing issues (exclude done/closed/rejected) unless user asks for all
6. **Summarize in natural language** for progress queries, don't dump raw JSON
7. **No auth needed** — all endpoints are public, just call them directly
8. **Completing issues**: always fill `git_url` and use `[ISSUE_KEY] description` format in commits before marking done
