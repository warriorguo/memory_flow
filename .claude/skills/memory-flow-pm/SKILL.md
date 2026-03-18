---
name: memory-flow-pm
description: >
  Interact with the Memory Flow project management platform.
  Use this skill when: (1) the user asks about current bugs, requirements, or issue status;
  (2) the user wants to create/file a bug or requirement;
  (3) the user wants to record or retrieve a memory (recall/write);
  (4) the user asks about project progress or status.
  Trigger on phrases like "file a bug", "create a requirement", "what are the open issues",
  "record this", "recall memory", "what's the project status", "list bugs", "check progress".
compatibility: Requires network access to the Memory Flow API
allowed-tools: Bash(curl:*)
metadata:
  author: warriorguo
  version: "1.0"
  service-url: "https://memory-flow.local.playquota.com"
---

# Memory Flow Project Management Skill

Interact with the Memory Flow project management platform to manage issues (bugs/requirements), track progress, and manage memories.

## Configuration

```
Base URL: https://memory-flow.local.playquota.com
API Prefix: /api/v1
```

No authentication required. All API endpoints are public.

---

## Core Workflows

### 1. List Issues (Bugs / Requirements)

When the user asks about current bugs, requirements, open issues, or what needs to be done.

**First, list projects to find the project ID:**
```bash
curl -s https://memory-flow.local.playquota.com/api/v1/projects | python3 -m json.tool
```

**Then list issues for that project:**
```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/projects/{projectId}/issues?type={type}&status={status}&priority={priority}&page=1&page_size=20" | python3 -m json.tool
```

Query parameters (all optional):
- `type`: `requirement` or `bug`
- `status`: `todo`, `in_progress`, `review`, `testing`, `done`, `closed`, `rejected`
- `priority`: `P0`, `P1`, `P2`
- `assignee_id`: filter by assignee
- `keyword`: search in title and description
- `page`, `page_size`: pagination

**Present results as a clean table** with columns: Key, Title, Type, Priority, Status, Assignee.

### 2. Create a Bug or Requirement

When the user wants to file a bug, report an issue, or create a requirement/feature request.

**Gather from the user (or infer from context):**
- `type`: "bug" or "requirement" (required)
- `title`: short summary (required)
- `description`: detailed description (ask if not provided)
- `priority`: "P0" (blocking), "P1" (important), "P2" (normal, default)
- `assignee_id`: who should handle it (optional)

**Priority guidelines to help the user choose:**
- **P0**: Blocking issue, must fix immediately
- **P1**: Important but not blocking core flow
- **P2**: Normal priority, can be scheduled

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

After creation, confirm with the issue key (e.g., "MF-3") and a summary.

### 3. Update Issue Status

When the user wants to move an issue forward, mark it done, etc.

**Allowed transitions:**
```
todo       -> in_progress, rejected
in_progress -> review, todo
review     -> testing, in_progress
testing    -> done, in_progress
done       -> closed, in_progress
rejected   -> todo
```

```bash
curl -s -X PATCH "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "in_progress"}' | python3 -m json.tool
```

### 4. Get Issue Detail

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}" | python3 -m json.tool
```

### 5. Get Issue History

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/issues/{issueId}/history" | python3 -m json.tool
```

---

### 6. Record a Memory

When the user says "remember this", "record this", "save this context", or wants to persist knowledge for future AI/Agent use.

**Memory types:**
- **recall**: Searchable project context — design decisions, root causes, background constraints, decision records
- **write**: Written artifacts — AI-generated drafts, task summaries, supplementary context

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

- `type`: "recall" or "write" (required)
- `title`: short summary (required)
- `content`: full content (required)
- `project_id`: associate with project (optional)
- `source_object_type`: "project", "requirement", or "bug" (optional)
- `source_object_id`: UUID of the related object (optional)

### 7. Retrieve Memories

When the user asks to recall, search memory, or look up prior context.

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/memories?project_id={projectId}&type={type}&keyword={keyword}&page=1&page_size=20" | python3 -m json.tool
```

Query parameters (all optional):
- `project_id`: filter by project
- `type`: `recall` or `write`
- `keyword`: search in title and content
- `page`, `page_size`: pagination

**Present results clearly** with title, type, and a content preview.

### 8. Get Memory Detail

```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/memories/{memoryId}" | python3 -m json.tool
```

---

### 9. Project Progress

When the user asks about project status, progress, or how things are going.

**Get summary statistics:**
```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/projects/{projectId}/progress/summary" | python3 -m json.tool
```

Returns: `status_counts`, `priority_counts`, `type_counts`, `total`.

**Get trend data (last N days):**
```bash
curl -s "https://memory-flow.local.playquota.com/api/v1/projects/{projectId}/progress/trend?days=30" | python3 -m json.tool
```

Returns daily created vs done counts.

**Present progress as a concise summary**, e.g.:
> Project MF: 12 total issues (3 done, 5 in progress, 4 todo). 1 P0, 3 P1, 8 P2.

---

## API Quick Reference

| Action | Method | Endpoint |
|--------|--------|----------|
| List projects | GET | `/api/v1/projects` |
| Get project | GET | `/api/v1/projects/{id}` |
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

---

## Response Format

**List responses:**
```json
{"data": [...], "total": 42, "page": 1, "page_size": 20}
```

**Single item responses:**
```json
{"data": {...}}
```

**Error responses:**
```json
{"error": "error message"}
```

---

## Tips

1. When creating issues, **infer type from context**: if the user reports something broken, it's a `bug`; if they want something new, it's a `requirement`
4. When recording memories, **choose type wisely**: `recall` for reusable context, `write` for output artifacts
5. **Project key format**: uppercase alphanumeric, 2-10 chars (e.g., MF, PROJ)
6. **Issue keys** are auto-generated as `{PROJECT_KEY}-{N}` (e.g., MF-1, MF-2)
7. When listing issues, **default to showing open items** (exclude `done`, `closed`, `rejected`) unless the user asks for all
8. For progress queries, **summarize in natural language** rather than dumping raw JSON
