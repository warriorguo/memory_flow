# Memory Flow build targets.

BIN ?= memory_flow
MF_BIN ?= mf
PREFIX ?= /usr/local
DIST_SRC := frontend/dist
DIST_EMBED := backend/internal/web/dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: frontend embed-frontend standalone standalone-all app mf install-mf test clean

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

# Build and install the native macOS app (AppKit + WKWebView) to /Applications.
app:
	./macapp/build.sh

# Build the `mf` command-line client. It talks to whichever instance is live
# (remote server or local standalone), so it embeds no frontend and no database.
mf:
	cd backend && CGO_ENABLED=0 go build \
		-ldflags "-X github.com/warriorguo/memory_flow/backend/internal/mfcli.Version=$(VERSION)" \
		-o ../$(MF_BIN) ./cmd/mf
	@echo "built ./$(MF_BIN)  (try: ./$(MF_BIN) ctx)"

# Install `mf` onto PATH. Override the location with PREFIX, e.g.
#   make install-mf PREFIX=$$HOME   ->  ~/bin/mf
install-mf: mf
	@mkdir -p $(PREFIX)/bin 2>/dev/null || { echo "cannot create $(PREFIX)/bin — retry with sudo, or PREFIX=$$HOME"; exit 1; }
	install -m 0755 $(MF_BIN) $(PREFIX)/bin/$(MF_BIN) || { echo "cannot write $(PREFIX)/bin — retry with sudo, or PREFIX=$$HOME"; exit 1; }
	@echo "installed $(PREFIX)/bin/$(MF_BIN)"

test:
	cd backend && go test ./...

clean:
	rm -f $(BIN) $(MF_BIN)
	rm -rf $(DIST_EMBED)/assets $(DIST_EMBED)/*.html $(DIST_EMBED)/*.svg
