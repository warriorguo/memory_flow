package mfcli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

// assetPath builds the endpoint for an issue's assets, escaping the filename so
// spaces and non-ASCII names survive the URL.
func assetPath(issueKey, filename string) string {
	base := "/api/v1/issues/" + url.PathEscape(issueKey) + "/assets"
	if filename == "" {
		return base
	}
	return base + "/" + url.PathEscape(filename)
}

func contentTypeFor(filename string) string {
	if t := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))); t != "" {
		return t
	}
	return "application/octet-stream"
}

// cmdAssetAdd uploads one or more files to an issue.
func cmdAssetAdd(e *env, args []string) error {
	fs := newFlagSet("asset add")
	name := fs.String("name", "", "store under this filename instead of the file's own name")
	file := fs.String("file", "", "read content from this path ('-' for stdin)")
	overwrite := fs.Bool("overwrite", false, "replace an existing asset with the same filename")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return fmt.Errorf("usage: mf asset add <ISSUE_KEY> <path>… [--name NAME] [--overwrite]")
	}
	issueKey := pos[0]
	paths := pos[1:]

	if *file != "" {
		paths = append(paths, *file)
	}
	if len(paths) == 0 {
		return fmt.Errorf("nothing to upload: give one or more file paths, or --file - to read stdin")
	}
	if *name != "" && len(paths) > 1 {
		return fmt.Errorf("--name applies to a single file; drop it when uploading %d files", len(paths))
	}

	for _, path := range paths {
		content, filename, err := readAssetSource(path, *name)
		if err != nil {
			return err
		}

		endpoint := assetPath(issueKey, "") + query(map[string]string{
			"filename":  filename,
			"overwrite": boolQuery(*overwrite),
		})

		var asset model.IssueAsset
		raw, err := e.client.uploadBytes("POST", endpoint, content, contentTypeFor(filename), &asset)
		if err != nil {
			return err
		}
		if e.emitJSON(raw) {
			continue
		}
		e.printf("uploaded %s → %s (%s, %s)\n", filename, issueKey, humanSize(asset.SizeBytes), asset.MimeType)
	}
	return nil
}

// readAssetSource loads the bytes to upload and decides the filename they will
// be stored under. Stdin has no name of its own, so --name is required there.
func readAssetSource(path, override string) ([]byte, string, error) {
	if path == "-" {
		if override == "" {
			return nil, "", fmt.Errorf("--name is required when reading from stdin")
		}
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, "", fmt.Errorf("read stdin: %w", err)
		}
		return content, override, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	filename := override
	if filename == "" {
		filename = filepath.Base(path)
	}
	return content, filename, nil
}

func boolQuery(v bool) string {
	if v {
		return "true"
	}
	return ""
}

func cmdAssetList(e *env, args []string) error {
	fs := newFlagSet("asset list")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 1, "asset list <ISSUE_KEY>"); err != nil {
		return err
	}

	assets, raw, err := e.fetchAssets(pos[0])
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	if len(assets) == 0 {
		e.printf("no assets on %s\n", pos[0])
		return nil
	}

	rows := make([][]string, 0, len(assets))
	for _, a := range assets {
		rows = append(rows, []string{a.Filename, a.MimeType, humanSize(a.SizeBytes), shortTime(a.UpdatedAt)})
	}
	table(e.out, []string{"FILENAME", "TYPE", "SIZE", "UPDATED"}, rows)
	e.printf("\n%d asset(s) on %s\n", len(assets), pos[0])
	return nil
}

func (e *env) fetchAssets(issueKey string) ([]model.IssueAsset, []byte, error) {
	var assets []model.IssueAsset
	raw, err := e.client.get(assetPath(issueKey, ""), &assets)
	if err != nil {
		return nil, nil, err
	}
	return assets, raw, nil
}

// cmdAssetGet downloads one asset, or every asset on the issue with --all.
func cmdAssetGet(e *env, args []string) error {
	fs := newFlagSet("asset get")
	out := fs.String("out", "", "write here — a file path, a directory with --all, or '-' for stdout")
	outShort := fs.String("o", "", "shorthand for --out")
	all := fs.Bool("all", false, "download every asset on the issue")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}

	dest := *out
	if dest == "" {
		dest = *outShort
	}

	if *all {
		if err := need(pos, 1, "asset get <ISSUE_KEY> --all -o <DIR>"); err != nil {
			return err
		}
		return e.downloadAll(pos[0], dest)
	}
	if err := need(pos, 2, "asset get <ISSUE_KEY> <FILENAME> [-o <PATH>|-]"); err != nil {
		return err
	}
	return e.downloadOne(pos[0], pos[1], dest)
}

func (e *env) downloadOne(issueKey, filename, dest string) error {
	content, _, err := e.client.download(assetPath(issueKey, filename) + "?download=1")
	if err != nil {
		return err
	}

	if dest == "" {
		dest = filename
	}
	if dest == "-" {
		if looksBinary(content) && stdoutIsTerminal() {
			return fmt.Errorf("%s looks binary — redirect stdout or pass -o <PATH>", filename)
		}
		_, err := e.out.Write(content)
		return err
	}
	// A directory destination keeps the asset's own name.
	if info, statErr := os.Stat(dest); statErr == nil && info.IsDir() {
		dest = filepath.Join(dest, filename)
	}
	if err := os.WriteFile(dest, content, 0o644); err != nil {
		return err
	}
	e.printf("wrote %s (%s)\n", dest, humanSize(int64(len(content))))
	return nil
}

func (e *env) downloadAll(issueKey, dir string) error {
	if dir == "" || dir == "-" {
		return fmt.Errorf("--all writes files, so -o <DIR> is required")
	}
	assets, _, err := e.fetchAssets(issueKey)
	if err != nil {
		return err
	}
	if len(assets) == 0 {
		e.printf("no assets on %s\n", issueKey)
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	for _, a := range assets {
		content, _, err := e.client.download(assetPath(issueKey, a.Filename) + "?download=1")
		if err != nil {
			return err
		}
		path := filepath.Join(dir, a.Filename)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
		e.printf("wrote %s (%s)\n", path, humanSize(a.SizeBytes))
	}
	e.printf("\n%d asset(s) from %s → %s\n", len(assets), issueKey, dir)
	return nil
}

// cmdAssetReplace overwrites an existing asset's content, keeping its name so
// descriptions that reference it keep resolving.
func cmdAssetReplace(e *env, args []string) error {
	fs := newFlagSet("asset replace")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 3, "asset replace <ISSUE_KEY> <FILENAME> <PATH>"); err != nil {
		return err
	}
	issueKey, filename, path := pos[0], pos[1], pos[2]

	content, _, err := readAssetSource(path, filename)
	if err != nil {
		return err
	}

	var asset model.IssueAsset
	raw, err := e.client.uploadBytes("PUT", assetPath(issueKey, filename), content, contentTypeFor(filename), &asset)
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("replaced %s on %s (%s)\n", filename, issueKey, humanSize(asset.SizeBytes))
	return nil
}

func cmdAssetRemove(e *env, args []string) error {
	fs := newFlagSet("asset rm")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if err := need(pos, 2, "asset rm <ISSUE_KEY> <FILENAME> [--yes]"); err != nil {
		return err
	}
	issueKey, filename := pos[0], pos[1]

	if !*yes {
		ok, err := confirm(e, fmt.Sprintf("delete %s from %s?", filename, issueKey))
		if err != nil {
			return err
		}
		if !ok {
			e.printf("cancelled\n")
			return nil
		}
	}

	raw, err := e.client.del(assetPath(issueKey, filename))
	if err != nil {
		return err
	}
	if e.emitJSON(raw) {
		return nil
	}
	e.printf("deleted %s from %s\n", filename, issueKey)
	// The server tells us when the description still points at the file we just
	// removed; passing that on is the difference between a clean delete and a
	// broken reference nobody notices.
	if warning := deleteWarning(raw); warning != "" {
		e.printf("warning: %s\n", warning)
	}
	return nil
}

func deleteWarning(raw []byte) string {
	var env struct {
		Data struct {
			Warning string `json:"warning"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return ""
	}
	return env.Data.Warning
}

// confirm asks for a y/N on stdin. A non-interactive stdin (a pipe, a CI run)
// answers no rather than blocking, so --yes is the scripted path.
func confirm(e *env, prompt string) (bool, error) {
	if !stdinIsTerminal() {
		return false, fmt.Errorf("stdin is not a terminal — pass --yes to confirm")
	}
	fmt.Fprintf(e.out, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func stdinIsTerminal() bool  { return isTerminal(os.Stdin) }
func stdoutIsTerminal() bool { return isTerminal(os.Stdout) }

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// looksBinary reports whether content would garble a terminal — a NUL byte or
// invalid UTF-8 in the leading window is enough to refuse writing it to stdout.
func looksBinary(content []byte) bool {
	window := content
	if len(window) > 8000 {
		window = window[:8000]
	}
	for _, b := range window {
		if b == 0 {
			return true
		}
	}
	return !utf8.Valid(window)
}

// annotateAssetRefs marks every asset: reference in a description so a reader
// can match it against the Assets block below, and flags the ones that name a
// file the issue does not have.
func annotateAssetRefs(description string, assets []model.IssueAsset) string {
	refs := service.AssetRefs(description)
	if len(refs) == 0 {
		return description
	}

	known := make(map[string]bool, len(assets))
	for _, a := range assets {
		known[a.Filename] = true
	}

	annotated := description
	for _, ref := range refs {
		resolved, ok := service.ResolveAssetRef(ref, known)
		note := "  ⚠ missing asset"
		if ok {
			note = "  (asset)"
		}
		// Replace the reference as written, keeping any trailing punctuation the
		// capture picked up outside the annotation.
		suffix := strings.TrimPrefix(ref, resolved)
		annotated = strings.ReplaceAll(annotated,
			service.AssetRefScheme+ref,
			service.AssetRefScheme+resolved+note+suffix)
	}
	return annotated
}

// printAssets renders the Assets block of `mf issue show`.
func printAssets(e *env, assets []model.IssueAsset) {
	if len(assets) == 0 {
		return
	}
	e.printf("\nAssets:\n")
	rows := make([][]string, 0, len(assets))
	for _, a := range assets {
		rows = append(rows, []string{"  " + a.Filename, a.MimeType, humanSize(a.SizeBytes)})
	}
	table(e.out, nil, rows)
}

// withAssets splices the asset list into an issue's JSON envelope, so
// `mf issue show --json` describes the issue and its attachments in one object.
func withAssets(raw []byte, assets []model.IssueAsset) []byte {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return raw
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(env["data"], &data); err != nil {
		return raw
	}
	if assets == nil {
		assets = []model.IssueAsset{}
	}
	encoded, err := json.Marshal(assets)
	if err != nil {
		return raw
	}
	data["assets"] = encoded
	if env["data"], err = json.Marshal(data); err != nil {
		return raw
	}
	merged, err := json.Marshal(env)
	if err != nil {
		return raw
	}
	return merged
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(n)/float64(div), "KMGT"[exp])
}
