// Package mfcli implements `mf`, the command-line client for the Memory Flow
// API. It exists so that agents and humans drive the platform through stable
// verbs (`mf issue done MF-1`) rather than hand-assembled curl pipelines.
package mfcli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// env carries everything a command needs: the API client, where to write, and
// whether the caller asked for raw JSON.
type env struct {
	client *Client
	out    io.Writer
	errOut io.Writer
	asJSON bool
}

// Main runs the CLI and returns the process exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	opts, rest, err := extractGlobals(args)
	if err != nil {
		fmt.Fprintln(stderr, "mf:", err)
		return 2
	}

	if len(rest) == 0 || rest[0] == "help" || rest[0] == "-h" || rest[0] == "--help" {
		topic := ""
		if len(rest) > 1 {
			topic = rest[1]
		}
		printUsage(stdout, topic)
		return 0
	}
	if rest[0] == "version" {
		fmt.Fprintln(stdout, "mf "+Version)
		return 0
	}

	client := NewClient(opts.url, opts.timeout)
	client.NoCache = opts.refresh
	e := &env{client: client, out: stdout, errOut: stderr, asJSON: opts.asJSON}

	if err := dispatch(e, rest); err != nil {
		fmt.Fprintln(stderr, "mf:", err)
		return 1
	}
	return 0
}

// Version is stamped at build time via -ldflags.
var Version = "dev"

type globalOpts struct {
	url     string
	timeout time.Duration
	asJSON  bool
	refresh bool
}

// extractGlobals pulls the flags that apply to every command out of the
// argument list, wherever they appear, so subcommands see only their own flags.
func extractGlobals(args []string) (globalOpts, []string, error) {
	opts := globalOpts{timeout: defaultTimeout}
	var rest []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")

		// takeValue reads the flag's argument, from --flag=value or the next token.
		takeValue := func() (string, error) {
			if hasValue {
				return value, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", name)
			}
			i++
			return args[i], nil
		}

		switch name {
		case "--url":
			v, err := takeValue()
			if err != nil {
				return opts, nil, err
			}
			opts.url = v
		case "--timeout":
			v, err := takeValue()
			if err != nil {
				return opts, nil, err
			}
			secs, err := strconv.Atoi(v)
			if err != nil {
				return opts, nil, fmt.Errorf("--timeout must be a number of seconds")
			}
			opts.timeout = time.Duration(secs) * time.Second
		case "--json":
			opts.asJSON = true
		case "--refresh":
			opts.refresh = true
		default:
			rest = append(rest, arg)
		}
	}
	return opts, rest, nil
}

func dispatch(e *env, args []string) error {
	switch args[0] {
	case "ctx", "context":
		return cmdCtx(e, args[1:])
	case "endpoint":
		return cmdEndpoint(e, args[1:])
	case "projects":
		return cmdProjectList(e, args[1:])
	case "project":
		return dispatchProject(e, args[1:])
	case "issues":
		return cmdIssueList(e, args[1:])
	case "issue":
		return dispatchIssue(e, args[1:])
	case "memories":
		return cmdMemoryList(e, args[1:])
	case "memory":
		return dispatchMemory(e, args[1:])
	case "tags":
		return cmdTagList(e, args[1:])
	case "tag":
		return dispatchTag(e, args[1:])
	default:
		return fmt.Errorf("unknown command %q (run `mf help`)", args[0])
	}
}

func dispatchProject(e *env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mf project <list|show|create|update|progress|archive> …")
	}
	switch args[0] {
	case "list":
		return cmdProjectList(e, args[1:])
	case "show", "get":
		return cmdProjectShow(e, args[1:])
	case "create":
		return cmdProjectCreate(e, args[1:])
	case "update":
		return cmdProjectUpdate(e, args[1:])
	case "progress":
		return cmdProjectProgress(e, args[1:])
	case "archive":
		return cmdProjectArchive(e, args[1:])
	default:
		return fmt.Errorf("unknown `mf project` subcommand %q", args[0])
	}
}

func dispatchIssue(e *env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mf issue <list|show|create|update|status|start|done|attach-git|history|dep|tag> …")
	}
	switch args[0] {
	case "list", "ls":
		return cmdIssueList(e, args[1:])
	case "show", "get":
		return cmdIssueShow(e, args[1:])
	case "create", "new":
		return cmdIssueCreate(e, args[1:])
	case "update", "edit":
		return cmdIssueUpdate(e, args[1:])
	case "status":
		return cmdIssueStatus(e, args[1:])
	case "start":
		return cmdIssueStart(e, args[1:])
	case "done":
		return cmdIssueDone(e, args[1:])
	case "attach-git":
		return cmdIssueAttachGit(e, args[1:])
	case "history":
		return cmdIssueHistory(e, args[1:])
	case "priority":
		return cmdIssuePriority(e, args[1:])
	case "tag":
		return cmdIssueTag(e, args[1:])
	case "untag":
		return cmdIssueUntag(e, args[1:])
	case "dep", "deps":
		return dispatchDep(e, args[1:])
	default:
		return fmt.Errorf("unknown `mf issue` subcommand %q", args[0])
	}
}

func dispatchDep(e *env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mf issue dep <add|list|rm|tree> <ISSUE_KEY> …")
	}
	switch args[0] {
	case "add":
		return cmdDepAdd(e, args[1:])
	case "list", "ls":
		return cmdDepList(e, args[1:])
	case "rm", "delete":
		return cmdDepRemove(e, args[1:])
	case "tree":
		return cmdDepTree(e, args[1:])
	default:
		return fmt.Errorf("unknown `mf issue dep` subcommand %q", args[0])
	}
}

func dispatchMemory(e *env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mf memory <add|search|show|update|rm> …")
	}
	switch args[0] {
	case "add", "create":
		return cmdMemoryAdd(e, args[1:])
	case "search", "list", "ls":
		return cmdMemoryList(e, args[1:])
	case "show", "get":
		return cmdMemoryShow(e, args[1:])
	case "update", "edit":
		return cmdMemoryUpdate(e, args[1:])
	case "rm", "delete":
		return cmdMemoryRemove(e, args[1:])
	default:
		return fmt.Errorf("unknown `mf memory` subcommand %q", args[0])
	}
}

func dispatchTag(e *env, args []string) error {
	if len(args) == 0 {
		return cmdTagList(e, nil)
	}
	switch args[0] {
	case "list", "ls":
		return cmdTagList(e, args[1:])
	case "create", "add":
		return cmdTagCreate(e, args[1:])
	default:
		return fmt.Errorf("unknown `mf tag` subcommand %q", args[0])
	}
}

// --- shared command plumbing ----------------------------------------------

// parseArgs splits leading positional arguments from flags, then parses the
// flags. Positionals must come first: `mf issue create MF --title x`.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	i := 0
	for ; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") && args[i] != "-" {
			break
		}
		positional = append(positional, args[i])
	}
	if err := fs.Parse(args[i:]); err != nil {
		return nil, err
	}
	// Anything left after flag parsing is a positional in the wrong place.
	if extra := fs.Args(); len(extra) > 0 {
		return nil, fmt.Errorf("unexpected argument %q (positional arguments must come before flags)", extra[0])
	}
	return positional, nil
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// need checks the positional-argument count, producing a usage error naming the
// command when it does not match.
func need(positional []string, n int, usage string) error {
	if len(positional) < n {
		return fmt.Errorf("usage: mf %s", usage)
	}
	if len(positional) > n {
		return fmt.Errorf("unexpected argument %q\nusage: mf %s", positional[n], usage)
	}
	return nil
}

// emitJSON pretty-prints a raw API response and reports whether it did, so
// commands can `if e.emitJSON(raw) { return nil }` before rendering text.
func (e *env) emitJSON(raw json.RawMessage) bool {
	if !e.asJSON {
		return false
	}
	var pretty any
	if err := json.Unmarshal(raw, &pretty); err != nil {
		fmt.Fprintln(e.out, string(raw))
		return true
	}
	enc := json.NewEncoder(e.out)
	enc.SetIndent("", "  ")
	enc.Encode(pretty)
	return true
}

func (e *env) printf(format string, a ...any) {
	fmt.Fprintf(e.out, format, a...)
}

// readContent returns inline text, or the contents of a file when the inline
// value is empty and a path is given. "-" reads stdin, which is how multi-line
// descriptions arrive without shell-quoting gymnastics.
func readContent(inline, path string) (string, error) {
	if inline != "" {
		return inline, nil
	}
	if path == "" {
		return "", nil
	}
	if path == "-" {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(raw), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// strPtr returns a pointer for a set flag, or nil for an unset one, matching
// the API's "omit means unchanged" update semantics.
func strPtr(fs *flag.FlagSet, name string, value *string) *string {
	if !flagWasSet(fs, name) {
		return nil
	}
	return value
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}
