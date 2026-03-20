#!/usr/bin/env python3
"""
dev-agent: Claude Agent that pulls open issues from Memory Flow and resolves them.

Usage:
  python dev_agent.py                        # work on all todo issues (current project)
  python dev_agent.py --issue MF-6           # work on a specific issue
  python dev_agent.py --project ORT          # work on a different project
  python dev_agent.py --list                 # list open issues, don't work on them
  python dev_agent.py --issue MF-6 --cicd    # also trigger CI/CD after done
"""

import anyio
import argparse
import os
import subprocess
import sys

import httpx
from claude_agent_sdk import (
    ClaudeAgentOptions,
    ResultMessage,
    SystemMessage,
    query,
)

MEMORY_FLOW_BASE = "https://memory-flow.local.playquota.com/api/v1"
CICD_BASE = "https://cicd.local.playquota.com/api"

PRIORITY_ORDER = {"P0": 0, "P1": 1, "P2": 2}


# ---------------------------------------------------------------------------
# Memory Flow API helpers
# ---------------------------------------------------------------------------


async def get_project(key: str) -> dict:
    async with httpx.AsyncClient() as client:
        resp = await client.get(f"{MEMORY_FLOW_BASE}/projects")
        resp.raise_for_status()
        projects = resp.json()["data"]
    project = next((p for p in projects if p["key"] == key), None)
    if not project:
        raise SystemExit(f"Project '{key}' not found. Available: {[p['key'] for p in projects]}")
    return project


async def get_open_issues(project_id: str) -> list[dict]:
    async with httpx.AsyncClient() as client:
        resp = await client.get(
            f"{MEMORY_FLOW_BASE}/projects/{project_id}/issues",
            params={"status": "todo", "page_size": 20},
        )
        resp.raise_for_status()
    return resp.json()["data"]


async def get_issue_by_key(key: str) -> dict:
    async with httpx.AsyncClient() as client:
        resp = await client.get(f"{MEMORY_FLOW_BASE}/issues", params={"key": key})
        resp.raise_for_status()
    return resp.json()["data"]


async def get_project_by_id(project_id: str) -> dict:
    async with httpx.AsyncClient() as client:
        resp = await client.get(f"{MEMORY_FLOW_BASE}/projects/{project_id}")
        resp.raise_for_status()
    return resp.json()["data"]


async def transition_issue(issue_id: str, status: str) -> None:
    async with httpx.AsyncClient() as client:
        resp = await client.patch(
            f"{MEMORY_FLOW_BASE}/issues/{issue_id}/status",
            json={"status": status},
        )
        resp.raise_for_status()


async def update_issue(issue_id: str, **fields) -> None:
    async with httpx.AsyncClient() as client:
        resp = await client.put(
            f"{MEMORY_FLOW_BASE}/issues/{issue_id}",
            json=fields,
        )
        resp.raise_for_status()


async def file_issue(project_id: str, issue_type: str, title: str, description: str, priority: str = "P2") -> dict:
    """File a new bug or requirement discovered during development."""
    async with httpx.AsyncClient() as client:
        resp = await client.post(
            f"{MEMORY_FLOW_BASE}/projects/{project_id}/issues",
            json={
                "type": issue_type,
                "title": title,
                "description": description,
                "priority": priority,
            },
        )
        resp.raise_for_status()
    return resp.json()["data"]


# ---------------------------------------------------------------------------
# CI/CD helpers
# ---------------------------------------------------------------------------


async def trigger_cicd_build(app_name: str, commit_sha: str) -> dict | None:
    """Find the app by name and trigger a build."""
    async with httpx.AsyncClient(timeout=30) as client:
        try:
            resp = await client.get(f"{CICD_BASE}/apps")
            resp.raise_for_status()
            apps = resp.json()
            app = next((a for a in apps if a["name"] == app_name), None)
            if not app:
                print(f"  [CI/CD] App '{app_name}' not found, skipping build.")
                return None

            resp = await client.post(
                f"{CICD_BASE}/apps/{app['id']}/releases",
                json={"branch": "main", "commit_sha": commit_sha},
            )
            resp.raise_for_status()
            release = resp.json()
            print(f"  [CI/CD] Build triggered: release #{release['id']} (status: {release['status']})")
            return release
        except httpx.HTTPError as e:
            print(f"  [CI/CD] Warning: {e}")
            return None


# ---------------------------------------------------------------------------
# Git helpers
# ---------------------------------------------------------------------------


def get_git_info(cwd: str) -> tuple[str, str]:
    """Returns (commit_sha, remote_url)."""
    sha = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=cwd, text=True).strip()
    remote = subprocess.check_output(["git", "remote", "get-url", "origin"], cwd=cwd, text=True).strip()
    if remote.endswith(".git"):
        remote = remote[:-4]
    return sha, remote


def has_new_commits(cwd: str, sha_before: str) -> bool:
    current = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=cwd, text=True).strip()
    return current != sha_before


# ---------------------------------------------------------------------------
# Core: resolve one issue with Claude Agent
# ---------------------------------------------------------------------------


async def resolve_issue(issue: dict, project: dict, cwd: str, trigger_cicd: bool = False) -> None:
    issue_key = issue["issue_key"]
    print(f"\n{'='*60}")
    print(f"[{issue_key}] {issue['priority']} {issue['type'].upper()}: {issue['title']}")
    print(f"{'='*60}")

    # Record SHA before agent starts
    sha_before, _ = get_git_info(cwd)

    # Transition to in_progress
    await transition_issue(issue["id"], "in_progress")

    system_prompt = f"""You are a software developer working on the '{project['name']}' project.
Repository: {cwd}

When you encounter something broken, missing, or unreasonable during development
(e.g. API doesn't match spec, docs missing, design flaw), file it as a bug or
requirement using the Memory Flow API instead of silently working around it:

  POST {MEMORY_FLOW_BASE}/projects/{project['id']}/issues
  Content-Type: application/json
  Body: {{"type": "bug"|"requirement", "title": "...", "description": "...", "priority": "P0"|"P1"|"P2"}}

Criteria for filing:
- bug: something is broken or behaves incorrectly
- requirement: something is missing or needs to be added
- Only file if it's a real blocker or design issue, not a minor style nit

Commit format: [{issue_key}] <short description>
"""

    prompt = f"""Work on the following issue:

Issue Key: {issue_key}
Title: {issue['title']}
Type: {issue['type']}
Priority: {issue['priority']}
Description:
{issue.get('description') or '(no description)'}

Steps:
1. Read the relevant code to understand the current state.
2. Implement the required changes.
3. Run any existing tests if applicable (e.g. `go test ./...` for Go, `npm test` for JS).
4. Commit all changes with message: [{issue_key}] <description>
5. Report what was done.

If you encounter a blocking problem that needs to be filed as a separate issue,
use the Memory Flow API (instructions in your system prompt) before continuing.
"""

    result = None
    async for message in query(
        prompt=prompt,
        options=ClaudeAgentOptions(
            cwd=cwd,
            allowed_tools=["Read", "Write", "Edit", "Bash", "Glob", "Grep"],
            permission_mode="acceptEdits",
            system_prompt=system_prompt,
            max_turns=80,
        ),
    ):
        if isinstance(message, SystemMessage) and message.subtype == "init":
            print(f"  Session: {message.data.get('session_id', 'unknown')}")
        elif isinstance(message, ResultMessage):
            result = message.result

    if result:
        print(f"\nAgent finished: {result[:200]}{'...' if len(result) > 200 else ''}")

    # Check if agent made any commits
    if not has_new_commits(cwd, sha_before):
        print(f"  Warning: no new commits detected for {issue_key}. Marking suspended.")
        await transition_issue(issue["id"], "suspended")
        return

    # Get commit info and update issue
    sha_after, remote_url = get_git_info(cwd)
    git_url = f"{remote_url}/commit/{sha_after}"
    await update_issue(issue["id"], git_url=git_url)
    print(f"  git_url: {git_url}")

    # Optionally trigger CI/CD
    if trigger_cicd and project.get("git_url"):
        app_name = project["name"]
        await trigger_cicd_build(app_name, sha_after)

    # Mark done
    await transition_issue(issue["id"], "done")
    print(f"  ✓ {issue_key} marked as done")


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


async def main() -> None:
    parser = argparse.ArgumentParser(description="Dev Agent — resolves Memory Flow issues autonomously")
    parser.add_argument("--project", default="MF", help="Project key (default: MF)")
    parser.add_argument("--issue", help="Work on a specific issue key (e.g. MF-6)")
    parser.add_argument("--list", action="store_true", help="List open issues and exit")
    parser.add_argument("--cicd", action="store_true", help="Trigger CI/CD build after completing each issue")
    parser.add_argument("--cwd", default=None, help="Working directory (default: git repo root)")
    args = parser.parse_args()

    # Resolve working directory
    cwd = args.cwd or subprocess.check_output(
        ["git", "rev-parse", "--show-toplevel"], text=True
    ).strip()
    cwd = os.path.abspath(cwd)
    print(f"Working directory: {cwd}")

    if args.issue:
        # Single issue mode
        issue = await get_issue_by_key(args.issue)
        project = await get_project_by_id(issue["project_id"])
        if args.list:
            print(f"[{issue['issue_key']}] {issue['priority']} {issue['status']}: {issue['title']}")
            return
        await resolve_issue(issue, project, cwd, trigger_cicd=args.cicd)
    else:
        # Batch mode: pull all todo issues for the project
        project = await get_project(args.project)
        issues = await get_open_issues(project["id"])

        if not issues:
            print(f"No open (todo) issues found for project {args.project}.")
            return

        print(f"Found {len(issues)} open issue(s) for {project['name']}:")
        for iss in sorted(issues, key=lambda x: PRIORITY_ORDER.get(x["priority"], 99)):
            print(f"  [{iss['issue_key']}] {iss['priority']} - {iss['title']}")

        if args.list:
            return

        # Work through issues by priority
        for iss in sorted(issues, key=lambda x: PRIORITY_ORDER.get(x["priority"], 99)):
            await resolve_issue(iss, project, cwd, trigger_cicd=args.cicd)


if __name__ == "__main__":
    anyio.run(main)
