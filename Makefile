# Memory Flow build targets.

BIN ?= memory_flow
DIST_SRC := frontend/dist
DIST_EMBED := backend/internal/web/dist

.PHONY: frontend embed-frontend standalone standalone-all test clean

# Build the web UI.
frontend:
	cd frontend && npm install && npm run build

# Copy the built frontend into the Go embed directory (required before building
# the standalone binary so the UI is bundled).
embed-frontend:
	@test -f $(DIST_SRC)/index.html || { echo "missing $(DIST_SRC) — run 'make frontend' first"; exit 1; }
	rm -rf $(DIST_EMBED)
	mkdir -p $(DIST_EMBED)
	cp -R $(DIST_SRC)/. $(DIST_EMBED)/
	touch $(DIST_EMBED)/.gitkeep

# Build the standalone single binary (embeds the current frontend/dist).
# Produces a CGo-free static binary (pure-Go SQLite via modernc).
standalone: embed-frontend
	cd backend && CGO_ENABLED=0 go build -o ../$(BIN) ./cmd/standalone
	@echo "built ./$(BIN)  (run: ./$(BIN) serve)"

# Build frontend then the standalone binary in one step.
standalone-all: frontend standalone

test:
	cd backend && go test ./...

clean:
	rm -f $(BIN)
	rm -rf $(DIST_EMBED)/assets $(DIST_EMBED)/*.html $(DIST_EMBED)/*.svg
