// Command standalone is the single-binary build of Memory Flow. It runs an
// embedded SQLite database, serves the API and the bundled web UI on localhost,
// and can sync its data to/from a remote (PostgreSQL-backed) server.
//
// Usage:
//
//	standalone                      # serve (default)
//	standalone serve                # serve the API + web UI on 127.0.0.1:$PORT
//	standalone sync push --server URL   # merge local data into the server
//	standalone sync pull --server URL   # merge server data into local
//
// Environment:
//
//	PORT               listen port (default 8080)
//	MEMORY_FLOW_DATA   path to the SQLite file (default ~/.memory_flow/data.db)
//	MAX_ASSET_BYTES    per-asset size cap in bytes (default 32MB)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/handler"
	"github.com/warriorguo/memory_flow/backend/internal/repository"
	"github.com/warriorguo/memory_flow/backend/internal/service"
	"github.com/warriorguo/memory_flow/backend/internal/syncclient"
	"github.com/warriorguo/memory_flow/backend/internal/synccore"
	"github.com/warriorguo/memory_flow/backend/internal/web"
	sqlitemigrations "github.com/warriorguo/memory_flow/backend/migrations_sqlite"
)

func main() {
	log.SetFlags(0)
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "serve":
		runServe()
	case "sync":
		runSync(args[1:])
	case "-h", "--help", "help":
		printUsage()
	default:
		log.Fatalf("unknown command %q (try: serve, sync)", cmd)
	}
}

func printUsage() {
	fmt.Println(`Memory Flow standalone

Commands:
  serve                                  Serve API + web UI on 127.0.0.1:$PORT (default 8080)
  sync push --server <url> [--token T]   Merge local data into the remote server
  sync pull --server <url> [--token T]   Merge remote server data into local

Env:
  PORT             listen port (default 8080)
  MEMORY_FLOW_DATA SQLite file path (default ~/.memory_flow/data.db)
  SYNC_TOKEN       shared secret for sync (used if --token is omitted)
  MAX_ASSET_BYTES  per-asset size cap in bytes (default 32MB)`)
}

// openDB resolves the data path, opens SQLite, and runs migrations.
func openDB() (database.DB, error) {
	path, err := dataPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	raw, err := database.OpenSQLiteDB(path)
	if err != nil {
		return nil, err
	}
	if err := database.MigrateSQLite(raw, sqlitemigrations.FS); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	log.Printf("using database %s", path)
	return database.WrapSQLite(raw), nil
}

// endpointFilePath is a fixed, well-known location holding the base URL the
// running standalone is serving on, so external tools can find it regardless of
// the (possibly random) port.
func endpointFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".memory_flow", "endpoint")
}

func writeEndpointFile(url string) {
	p := endpointFilePath()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(url+"\n"), 0o644); err != nil {
		log.Printf("warning: could not write endpoint file: %v", err)
	}
}

func removeEndpointFile() {
	if p := endpointFilePath(); p != "" {
		_ = os.Remove(p)
	}
}

// maxAssetBytes reads the per-asset size cap, falling back to the service
// default when unset or unparseable.
func maxAssetBytes() int64 {
	v := os.Getenv("MAX_ASSET_BYTES")
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		log.Printf("warning: ignoring MAX_ASSET_BYTES=%q: %v", v, err)
		return 0
	}
	return n
}

func dataPath() (string, error) {
	if p := os.Getenv("MEMORY_FLOW_DATA"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".memory_flow", "data.db"), nil
}

func runServe() {
	db, err := openDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	token := os.Getenv("SYNC_TOKEN")
	apiRouter := buildAPIRouter(db, token)

	mux := http.NewServeMux()
	// Standalone-only convenience endpoints so the web UI can trigger sync
	// same-origin (the binary does the outbound HTTP to the remote).
	mux.HandleFunc("/api/v1/sync/push", localSyncHandler(db, token, syncclient.Push))
	mux.HandleFunc("/api/v1/sync/pull", localSyncHandler(db, token, syncclient.Pull))
	mux.Handle("/api/", apiRouter)

	if spa, ok := web.Handler(); ok {
		mux.Handle("/", spa)
	} else {
		log.Println("warning: embedded frontend is empty (run the build step to bundle frontend/dist)")
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not bundled", http.StatusNotFound)
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "127.0.0.1:" + port

	// Advertise the live endpoint so external tools (e.g. the memory-flow-pm
	// agent skill) can discover the actual port — important because the native
	// macOS app launches on a random free port.
	writeEndpointFile("http://" + addr)
	defer removeEndpointFile()

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
		// Asset uploads move tens of megabytes; a JSON-sized read timeout would
		// cut them off.
		ReadTimeout: 120 * time.Second,
		// No WriteTimeout: sync export/import can stream large payloads.
		IdleTimeout: 60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Memory Flow standalone listening on http://%s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-done
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
}

// buildAPIRouter wires the shared repositories/services/handlers over db.
func buildAPIRouter(db database.DB, syncToken string) chi.Router {
	projectRepo := repository.NewProjectRepo(db)
	issueRepo := repository.NewIssueRepo(db)
	issueHistoryRepo := repository.NewIssueHistoryRepo(db)
	memoryRepo := repository.NewMemoryRepo(db)
	tagRepo := repository.NewTagRepo(db)
	depRepo := repository.NewDependencyRepo(db)
	assetRepo := repository.NewAssetRepo(db, repository.NewDBContentStore())

	projectSvc := service.NewProjectService(projectRepo)
	issueSvc := service.NewIssueService(issueRepo, projectRepo, issueHistoryRepo)
	progressSvc := service.NewProgressService(issueRepo)
	memorySvc := service.NewMemoryService(memoryRepo)
	depSvc := service.NewDependencyService(depRepo, issueRepo, projectRepo)
	assetSvc := service.NewAssetService(assetRepo, maxAssetBytes())

	resolver := handler.NewIDResolver(projectSvc, issueSvc)

	return handler.NewRouter(
		handler.NewProjectHandler(projectSvc, resolver),
		handler.NewIssueHandler(issueSvc, tagRepo, resolver),
		handler.NewProgressHandler(progressSvc, resolver),
		handler.NewMemoryHandler(memorySvc),
		handler.NewTagHandler(tagRepo, resolver),
		handler.NewDependencyHandler(depSvc, resolver),
		handler.NewSyncHandler(db, syncToken),
		handler.NewAssetHandler(assetSvc, issueSvc, resolver),
	)
}

// localSyncHandler exposes POST {"server_url": "...", "token": "..."} which runs
// a push/pull against the given remote and returns the merge result as JSON.
// This lets the bundled web UI trigger sync same-origin while the binary makes
// the outbound request to the remote server. If the request omits a token, the
// process-level defaultToken (SYNC_TOKEN env) is used.
func localSyncHandler(db database.DB, defaultToken string, fn func(context.Context, database.DB, string, string) (*synccore.ImportResult, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ServerURL string `json:"server_url"`
			Token     string `json:"token"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil || body.ServerURL == "" {
			http.Error(w, `{"error":"server_url is required"}`, http.StatusBadRequest)
			return
		}
		token := body.Token
		if token == "" {
			token = defaultToken
		}
		res, err := fn(r.Context(), db, body.ServerURL, token)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(res)
	}
}

func runSync(args []string) {
	if len(args) < 1 {
		log.Fatal("usage: standalone sync <push|pull> --server <url>")
	}
	direction := args[0]
	serverURL := parseFlag(args[1:], "--server")
	if serverURL == "" {
		log.Fatal("missing --server <url>")
	}
	// Token precedence: --token flag, then SYNC_TOKEN env.
	token := parseFlag(args[1:], "--token")
	if token == "" {
		token = os.Getenv("SYNC_TOKEN")
	}

	db, err := openDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	switch direction {
	case "push":
		res, err := syncclient.Push(ctx, db, serverURL, token)
		if err != nil {
			log.Fatalf("push failed: %v", err)
		}
		log.Printf("pushed to %s: applied=%d skipped=%d", serverURL, res.Applied, res.Skipped)
		reportErrors(res.Errors)
	case "pull":
		res, err := syncclient.Pull(ctx, db, serverURL, token)
		if err != nil {
			log.Fatalf("pull failed: %v", err)
		}
		log.Printf("pulled from %s: applied=%d skipped=%d", serverURL, res.Applied, res.Skipped)
		reportErrors(res.Errors)
	default:
		log.Fatalf("unknown sync direction %q (use push or pull)", direction)
	}
}

// parseFlag returns the value of a "--name value" or "--name=value" flag.
func parseFlag(args []string, name string) string {
	eq := name + "="
	for i := 0; i < len(args); i++ {
		if args[i] == name {
			if i+1 < len(args) {
				return args[i+1]
			}
		} else if strings.HasPrefix(args[i], eq) {
			return args[i][len(eq):]
		}
	}
	return ""
}

func reportErrors(errs []string) {
	if len(errs) == 0 {
		return
	}
	log.Printf("%d row(s) skipped due to conflicts:", len(errs))
	for _, e := range errs {
		log.Printf("  - %s", e)
	}
}
