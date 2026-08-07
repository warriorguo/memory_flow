package mfcli

import (
	"fmt"
	"io"
)

const rootUsage = `mf — command-line client for Memory Flow

Usage:
  mf <command> [subcommand] [args] [flags]

The instance is resolved automatically: --url / $MEMORY_FLOW_URL, else the
remote home server, else the local standalone app (~/.memory_flow/endpoint,
then http://127.0.0.1:8080). Run 'mf endpoint' to see which one is in use.

Commands:
  ctx                     Resolve the endpoint and list every project (start here)
  endpoint                Print the resolved base URL and how it was found
  projects                List projects
  project …               show | create | update | progress | archive
  issues <PROJECT_KEY>    List open issues in a project
  issue …                 show | create | update | status | start | done |
                          attach-git | history | priority | tag | dep
  memories                Search memories
  memory …                add | search | show | update | rm
  tags                    List tags
  tag create <NAME>       Create a tag

Global flags (accepted anywhere in the command line):
  --json                  Print the raw API response instead of formatted text
  --url <URL>             Pin the instance instead of auto-resolving
  --refresh               Ignore the cached endpoint and probe again
  --timeout <SECONDS>     Request timeout (default 30)

Run 'mf help <topic>' for details: issue, project, memory, tag, workflow.

Examples:
  mf ctx
  mf issues MF --priority P0
  mf issue show ORT-100
  mf issue attach-git ORT-100 5253083
  mf issue done ORT-100
`

const issueUsage = `mf issue — bugs and requirements

  mf issue list <PROJECT_KEY> [--status S] [--type bug|requirement] [--priority P0|P1|P2]
                              [--assignee ID] [--keyword TEXT] [--all] [--limit N] [--page N]
      Open issues only by default; --all or an explicit --status includes done/closed/rejected.

  mf issue show <ISSUE_KEY> [--deps] [--history]
  mf issue create <PROJECT_KEY> --type <bug|requirement> --title "…" [--desc "…"|--desc-file FILE]
                                [--priority P1] [--assignee ID] [--source S] [--version V]
                                [--git-url URL] [--pr-url URL] [--doc-url URL]
  mf issue update <ISSUE_KEY> [--title "…"] [--desc "…"|--desc-file FILE] [--priority P0]
                              [--assignee ID] [--type T] [--git-url URL] [--pr-url URL] …
  mf issue status <ISSUE_KEY> <STATUS> [--direct]
  mf issue start <ISSUE_KEY>
  mf issue done <ISSUE_KEY> [--git <sha|url>] [--repo DIR] [--force]
  mf issue attach-git <ISSUE_KEY> [<sha|url>] [--repo DIR] [--repo-url URL] [--pr <NUM|URL>]
  mf issue history <ISSUE_KEY>
  mf issue priority <ISSUE_KEY>          Effective priority, including inherited
  mf issue tag <ISSUE_KEY> <NAME>…       Creates tags that do not exist yet
  mf issue untag <ISSUE_KEY> <NAME>
  mf issue dep add <ISSUE_KEY> <TARGET_KEY> [--type depends_on|blocks] [--severity critical|recommended]
  mf issue dep list|tree <ISSUE_KEY>
  mf issue dep rm <ISSUE_KEY> <DEPENDENCY_ID>

--desc-file - reads the description from stdin, which avoids shell-quoting a
long multi-line body:

  mf issue create MF --type bug --title "Search returns nothing" --desc-file - <<'EOF'
  Steps to reproduce…
  EOF
`

const projectUsage = `mf project — projects

  mf projects [--status active|paused|archived] [--name TEXT] [--limit N]
  mf project show <PROJECT_KEY>
  mf project create <KEY> --name "…" [--summary "…"] [--desc "…"|--desc-file FILE]
                          [--design-principles "…"] [--git-url URL] [--cicd-url URL]
                          [--doc-url URL] [--owner ID]
  mf project update <PROJECT_KEY> [--name "…"] [--summary "…"] [--status active|paused|archived]
                                  [--git-url URL] [--next-issue-number N] …
  mf project progress <PROJECT_KEY> [--trend <DAYS>]
  mf project archive <PROJECT_KEY>
`

const memoryUsage = `mf memory — recorded context

  mf memory add --title "…" --content "…"|--content-file FILE
                [--type recall|write] [--project KEY] [--issue ISSUE_KEY]
  mf memory search ["<KEYWORD>"] [--project KEY] [--type recall|write] [--limit N] [--full]
  mf memory show <MEMORY_ID>
  mf memory update <MEMORY_ID> [--title "…"] [--content "…"|--content-file FILE]
  mf memory rm <MEMORY_ID>

  recall  reusable project context — design decisions, root causes, constraints
  write   produced artifacts — drafts, task summaries, supplementary context

--issue attaches the memory to an issue and infers its project.
`

const tagUsage = `mf tag — tags

  mf tags                                List every tag
  mf tag create <NAME> [--color '#1890ff']
  mf issue tag <ISSUE_KEY> <NAME>…       Attach tags, creating missing ones
  mf issue untag <ISSUE_KEY> <NAME>
`

const workflowUsage = `Issue workflow

Status graph (each hop is enforced by the API):

  todo        → in_progress, suspended, rejected
  in_progress → review, done, suspended, todo
  review      → testing, in_progress
  testing     → done, in_progress
  done        → closed, in_progress
  suspended   → todo
  rejected    → todo

'mf issue status' and 'mf issue done' walk intermediate hops for you: closing a
todo issue performs todo → in_progress → done. Pass --direct to require a single
legal hop instead.

Completing an issue:

  1. Commit with the issue key in the subject:  [MF-1] Add image rotation support
  2. Record the commit and close, in one step:  mf issue done MF-1 --git HEAD

'mf issue done' refuses to close an issue whose git_url is empty (--force
overrides). --git accepts a full URL, a short sha, or any commit-ish such as
HEAD; the commit URL is built from the project's git_url, falling back to the
origin remote of --repo (default: the current directory).
`

func printUsage(w io.Writer, topic string) {
	switch topic {
	case "issue", "issues":
		fmt.Fprint(w, issueUsage)
	case "project", "projects":
		fmt.Fprint(w, projectUsage)
	case "memory", "memories":
		fmt.Fprint(w, memoryUsage)
	case "tag", "tags":
		fmt.Fprint(w, tagUsage)
	case "workflow", "status", "done":
		fmt.Fprint(w, workflowUsage)
	default:
		fmt.Fprint(w, rootUsage)
	}
}
