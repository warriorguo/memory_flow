#!/bin/bash
# Build the native macOS Memory Flow.app (AppKit + WKWebView wrapper around the
# embedded Go standalone server). Produces /Applications/Memory Flow.app.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP="${APP:-/Applications/Memory Flow.app}"
GOBIN="$ROOT/memory_flow"

echo "==> Building standalone Go binary (embeds web UI)"
make -C "$ROOT" standalone >/dev/null

echo "==> Compiling Swift wrapper"
TMP="$(mktemp -d)"
swiftc -O -o "$TMP/MemoryFlow" "$ROOT/macapp/MemoryFlow.swift" \
  -framework Cocoa -framework WebKit

echo "==> Assembling bundle: $APP"
rm -rf "$APP"   # avoid stale-mtime / leftover files (ditto/mtime gotcha)
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
install -m 0755 "$TMP/MemoryFlow" "$APP/Contents/MacOS/MemoryFlow"
install -m 0755 "$GOBIN"          "$APP/Contents/Resources/memory_flow"

# App icon from the committed brain master (macapp/icon_master.png).
MASTER="$ROOT/macapp/icon_master.png"
if [ -f "$MASTER" ]; then
  IT="$TMP/AppIcon.iconset"; mkdir -p "$IT"
  for s in 16 32 128 256 512; do
    sips -z $s $s          "$MASTER" --out "$IT/icon_${s}x${s}.png"    >/dev/null 2>&1 || true
    sips -z $((s*2)) $((s*2)) "$MASTER" --out "$IT/icon_${s}x${s}@2x.png" >/dev/null 2>&1 || true
  done
  iconutil -c icns "$IT" -o "$APP/Contents/Resources/AppIcon.icns" 2>/dev/null \
    && echo "   icon: AppIcon.icns from brain master" \
    || echo "   icon: skipped (iconutil failed)"
else
  echo "   icon: skipped (no macapp/icon_master.png — run macapp/make_icon.py)"
fi

cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key>             <string>Memory Flow</string>
  <key>CFBundleDisplayName</key>      <string>Memory Flow</string>
  <key>CFBundleIdentifier</key>       <string>com.warriorguo.memoryflow</string>
  <key>CFBundleVersion</key>          <string>1.0</string>
  <key>CFBundleShortVersionString</key><string>1.0</string>
  <key>CFBundlePackageType</key>      <string>APPL</string>
  <key>CFBundleExecutable</key>       <string>MemoryFlow</string>
  <key>CFBundleIconFile</key>         <string>AppIcon</string>
  <key>LSMinimumSystemVersion</key>   <string>11.0</string>
  <key>NSHighResolutionCapable</key>  <true/>
  <key>NSAppTransportSecurity</key>
  <dict><key>NSAllowsLocalNetworking</key><true/></dict>
</dict>
</plist>
PLIST

# Ad-hoc code sign so the WebView and networking run without Gatekeeper nags.
codesign --force --deep --sign - "$APP" >/dev/null 2>&1 || echo "   (codesign skipped)"
xattr -dr com.apple.quarantine "$APP" 2>/dev/null || true

rm -rf "$TMP"
echo "==> Done: $APP"
