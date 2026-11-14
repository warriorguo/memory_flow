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
  issue …                 show | context | create | similar | dup | update | status |
                          start | done | attach-git | history | priority | tag | dep
  epics <PROJECT_KEY>     List a project's epics with how far along each is
  epic …                  show | create | add | rm | progress | close
  asset …                 add | list | get | replace | rm  (files attached to an issue)
  comment …               add | list | rm  (discussion on an issue)
  memories                Search memories
  memory …                add | search | show | update | rm
  tags                    List tags
  tag …                   create | update  (colored labels; filter with 'mf issues --tag')

Global flags (accepted anywhere in the command line):
  --json                  Print the raw API response instead of formatted text
  --url <URL>             Pin the instance instead of auto-resolving
  --refresh               Ignore the cached endpoint and probe again
  --timeout <SECONDS>     Request timeout (default 30)
  --as <ID>               Who you are: authors comments, and decides whose
                          unread count 'mf issue show' reports
                          (default: $MF_ACTOR, else the OS user)

Run 'mf help <topic>' for details: issue, epic, project, asset, comment, memory,
tag, workflow.

Examples:
  mf ctx
  mf issues MF --priority P0
  mf issue show ORT-100
  mf issue attach-git ORT-100 5253083
  mf issue done ORT-100
`

const issueUsage = `mf issue — bugs and requirements

  mf issue list <PROJECT_KEY> [--status S] [--type bug|requirement|epic] [--priority P0|P1|P2]
                              [--assignee ID] [--keyword TEXT] [--tag NAME]… [--epic KEY]
                              [--all] [--limit N] [--page N]
  mf issue list --epic <EPIC_KEY> [--status S] [--all]
      With --epic and no project key: the epic's issues from every project.
      Open issues only by default; --all or an explicit --status includes done/closed/rejected/duplicate.
      Tags follow each title as [name] (colored on a terminal). --tag is repeatable
      and every given tag must match: --tag room --tag enemy-ai.

  mf issue show <ISSUE_KEY> [--deps] [--history] [--comments]
      Also lists the issue's assets, its recorded memories, and whether anyone
      has left comments you have not read. --comments prints the thread and
      marks it read. An issue in an epic also shows "epic KEY" and the epic's
      shared memories, marked *new / *updated since you last read them.
  mf issue context <ISSUE_KEY> [--start]
      Pick-up read: the issue, its epic (description, sibling issues, blockers),
      the epic's shared memories and the issue's own in full, assets and the
      comment thread. Marks the shared memories and comments read; --start also
      moves the issue to in_progress.
  mf issue create <PROJECT_KEY> --type <bug|requirement|epic> --title "…" [--desc "…"|--desc-file FILE]
                                [--priority P1] [--assignee ID] [--source S] [--version V]
                                [--git-url URL] [--pr-url URL] [--doc-url URL] [--tag NAME]…
                                [--epic EPIC_KEY]
      --epic files the issue straight into an epic. --tag is repeatable; tags that do not exist yet are created with a palette color.
      Never blocks on duplicates: when existing issues look like the same thing, they
      are listed after the "created" line under "Possible duplicates" (and in --json
      as similar_issues). --not-dup records that you checked them and this is separate.
  mf issue similar <PROJECT_KEY> --title "…" [--desc "…"|--desc-file FILE] [--type T]
  mf issue similar <ISSUE_KEY>
      Suspected duplicates of a draft (check before filing), or of an existing issue.
      Matches on shared specific identifiers (session ids, room/file ids, issue keys,
      backtick-quoted names) and close titles, over open issues and those closed in 30 days.
  mf issue update <ISSUE_KEY> [--title "…"] [--desc "…"|--desc-file FILE] [--priority P0]
                              [--assignee ID] [--type T] [--git-url URL] [--pr-url URL] …
  mf issue status <ISSUE_KEY> <STATUS> [--direct]
  mf issue dup <ISSUE_KEY> <TARGET_KEY>
      Close ISSUE as a duplicate of TARGET: status "duplicate" (filterable apart from
      rejected), ISSUE shows "dup of TARGET", TARGET lists it under "dups" and gets a
      comment. A TARGET that is itself a duplicate resolves to the issue it points at.
      Reopen with 'mf issue status <ISSUE_KEY> todo'.
  mf issue start <ISSUE_KEY>
  mf issue done <ISSUE_KEY> [--git <sha|url>] [--repo DIR] [--force]
  mf issue attach-git <ISSUE_KEY> [<sha|url>] [--repo DIR] [--repo-url URL] [--pr <NUM|URL>]
  mf issue history <ISSUE_KEY>
  mf issue comment <ISSUE_KEY> "…"       Shorthand for 'mf comment add'
  mf issue comments <ISSUE_KEY>          Shorthand for 'mf comment list'
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

const epicUsage = `mf epic — epics (an issue of type epic that groups other issues)

  mf epics <PROJECT_KEY> [--all]              Epics with done/total and %; --all adds closed ones
  mf epic show <EPIC_KEY>                     Description, issues, progress, dependencies among
                                              the issues, blocked issues, shared memories, assets
  mf epic create <PROJECT_KEY> --title "…" [--desc …] [--priority P1] [--tag NAME]…
  mf epic add <EPIC_KEY> <ISSUE_KEY>…         Issues may come from any project; one already in
                                              another epic moves (an issue has at most one epic)
  mf epic rm <ISSUE_KEY>…                     Take issues out of their epic
  mf epic progress <EPIC_KEY> [--trend DAYS]  Status/priority/type counts, blocked issues, trend
  mf epic close <EPIC_KEY>                    Mark done (no commit needed for an epic)

Membership is not a dependency: no ordering, no priority inheritance. Done
ratio counts done+closed over every issue except rejected and duplicate ones.
"Blocked" means a critical depends_on (or blocks) on an issue not done yet.

Shared memories: a memory recorded on the epic is visible from every issue in
it ('mf issue show', 'mf issue context', 'mf memory search --issue').

  mf memory add --epic <EPIC_OR_ISSUE_KEY> --title "…" --content "…"   [--new]
      Write a conclusion back to the epic. Refused when the epic already has a
      memory that seems to say the same thing — update that one instead
      (mf memory update <ID>), or pass --new if it really is separate.
  mf memory promote <MEMORY_ID> [--copy] [--force]
      Lift a memory recorded on an issue to the issue's epic.
`

const projectUsage = `mf project — projects

  mf projects [--status active|paused|archived] [--name TEXT] [--limit N]
  mf project show <PROJECT_KEY>
  mf project create <KEY> --name "…" [--summary "…"] [--desc "…"|--desc-file FILE]
                          [--design-principles "…"] [--git-url URL] [--cicd-url [LABEL=]URL]…
                          [--doc-url URL] [--owner ID]
  mf project update <PROJECT_KEY> [--name "…"] [--summary "…"] [--status active|paused|archived]
                                  [--git-url URL] [--cicd-url [LABEL=]URL]… [--next-issue-number N] …

  --cicd-url is repeatable, one per CI/CD app: --cicd-url server=URL --cicd-url admin=URL.
  On update it replaces the whole list; --cicd-url "" clears it.
  mf project progress <PROJECT_KEY> [--trend <DAYS>]
  mf project archive <PROJECT_KEY>
`

const assetUsage = `mf asset — files attached to an issue

  mf asset add <ISSUE_KEY> <path>… [--name NAME] [--overwrite]
  mf asset add <ISSUE_KEY> --file - --name <NAME>      Read the content from stdin
  mf asset list <ISSUE_KEY>
  mf asset get <ISSUE_KEY> <FILENAME> [-o <PATH>|-]    '-' writes to stdout
  mf asset get <ISSUE_KEY> --all -o <DIR>              Fetch every asset at once
  mf asset replace <ISSUE_KEY> <FILENAME> <PATH>
  mf asset rm <ISSUE_KEY> <FILENAME> [--yes]

Assets are addressed by filename, unique per issue, and can be anything: images,
video, audio, logs, sample data, design docs. Uploading over an existing name
fails rather than overwriting — pass --overwrite, or use 'replace', when that is
what you mean. A description can point at one with asset:<FILENAME>.

Assets vs. memories: a memory holds reusable project knowledge (a decision, a
root cause); an asset holds a file that is an input to, or an output of, the
work.

Typical loop:

  mf asset add OZX-12 ~/art/enemy_ref.png ~/audio/hit.wav   # attach the material
  mf asset get OZX-12 --all -o ./assets                     # pull it before coding
  mf asset add OZX-12 ./screenshots/result.png              # hand back the proof
  mf asset replace OZX-12 enemy_ref.png ~/art/enemy_v2.png  # art revised the file
`

const commentUsage = `mf comment — discussion on an issue

  mf comment add <ISSUE_KEY> "…" [--body-file FILE] [--author ID]
  mf comment add <ISSUE_KEY> --body-file -             Read the comment from stdin
  mf comment list <ISSUE_KEY> [--unread] [--no-mark]
  mf comment rm <ISSUE_KEY> <COMMENT_ID> [--yes]

'mf issue comment' and 'mf issue comments' are shorthands for add and list.

Comments are per-reader: listing them marks them read for whoever you are
(--as, else $MF_ACTOR, else the OS user), and 'mf issue show' reports how many
are still unread for that identity. Your own comments never count as unread.
Pass --no-mark to look without clearing the flag.

  mf issue show MF-1                       # "Comments: 3, 1 unread …"
  mf comment list MF-1                     # read them (clears the flag)
  mf comment list MF-1 --unread --no-mark  # peek at what is new
  mf comment add MF-1 "Repro'd on 1.4.2 — the crash is in the loader"

Comment, memory or history? A **comment** is a message to whoever reads the
issue next. A **memory** is knowledge worth keeping after the issue closes. The
**history** is the automatic record of field changes — nobody writes it.
`

const memoryUsage = `mf memory — recorded context

  mf memory add --title "…" --content "…"|--content-file FILE
                [--type recall|write] [--project KEY] [--issue ISSUE_KEY]
                [--epic EPIC_OR_ISSUE_KEY [--new]]
  mf memory search ["<KEYWORD>"] [--project KEY] [--type recall|write] [--limit N] [--full]
  mf memory search --issue <ISSUE_KEY> [--full]
      Everything the issue can see: its own memories and its epic's, labeled.
  mf memory promote <MEMORY_ID> [--copy] [--force]   Lift an issue memory to its epic
  mf memory show <MEMORY_ID>
  mf memory update <MEMORY_ID> [--title "…"] [--content "…"|--content-file FILE]
  mf memory rm <MEMORY_ID>

  recall  reusable project context — design decisions, root causes, constraints
  write   produced artifacts — drafts, task summaries, supplementary context

--issue attaches the memory to an issue and infers its project. --epic records
it on an epic (given the epic or any issue in it), shared with every issue of
the epic, after checking it does not restate an existing epic memory.
`

const tagUsage = `mf tag — tags

  mf tags                                List every tag with its color
  mf tag create <NAME> [--color '#1890ff']
  mf tag update <NAME> [--color '#1890ff'] [--name NEW_NAME]
  mf issue create <PROJECT_KEY> … --tag NAME [--tag NAME]…
  mf issue tag <ISSUE_KEY> <NAME>…       Attach tags, creating missing ones
  mf issue untag <ISSUE_KEY> <NAME>
  mf issues <PROJECT_KEY> --tag NAME [--tag NAME]…   Issues carrying every given tag

Every tag has a color: one left out is picked from a fixed palette, favoring
colors few tags use yet. Names match case-insensitively everywhere. Issue
lists show tags after the title — [room] [enemy-ai] — colored on a terminal
(set NO_COLOR to turn that off).
`

const workflowUsage = `Issue workflow

Status graph (each hop is enforced by the API):

  todo        → in_progress, suspended, rejected, duplicate
  in_progress → review, done, suspended, todo, duplicate
  review      → testing, in_progress, duplicate
  testing     → done, in_progress, duplicate
  done        → closed, in_progress
  suspended   → todo, duplicate
  rejected    → todo
  duplicate   → todo

"duplicate" is entered only with 'mf issue dup <KEY> <TARGET>', which records
the target; 'mf issue status <KEY> todo' reopens it and drops the link.

'mf issue status' and 'mf issue done' walk intermediate hops for you: closing a
todo issue performs todo → in_progress → done. Pass --direct to require a single
legal hop instead.

Completing an issue:

  1. Commit with the issue key in the subject:  [MF-1] Add image rotation support
  2. Record the commit and close, in one step:  mf issue done MF-1 --git HEAD

'mf issue done' refuses to close an issue whose git_url is empty (--force
overrides); epics are exempt. --git accepts a full URL, a short sha, or any commit-ish such as
HEAD; the commit URL is built from the project's git_url, falling back to the
origin remote of --repo (default: the current directory).
`

func printUsage(w io.Writer, topic string) {
	switch topic {
	case "issue", "issues":
		fmt.Fprint(w, issueUsage)
	case "project", "projects":
		fmt.Fprint(w, projectUsage)
	case "epic", "epics":
		fmt.Fprint(w, epicUsage)
	case "asset", "assets":
		fmt.Fprint(w, assetUsage)
	case "comment", "comments":
		fmt.Fprint(w, commentUsage)
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
