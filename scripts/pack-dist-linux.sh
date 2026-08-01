#!/usr/bin/env bash
# Pack self-contained Linux end-user folders (mirrors scripts/pack-dist.ps1):
#   dist-linux-amd64/      — WSL / generic Linux x86_64
#   dist-jetson-orin-nx/   — Jetson Orin NX (linux/arm64)
#
# Usage (from repo root or scripts/):
#   ./scripts/pack-dist-linux.sh
#   ./scripts/pack-dist-linux.sh --skip-frontend
#   ./scripts/pack-dist-linux.sh amd64          # only amd64
#   ./scripts/pack-dist-linux.sh jetson         # only Jetson arm64
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SKIP_FRONTEND=0
TARGETS=()

for arg in "$@"; do
  case "$arg" in
    --skip-frontend) SKIP_FRONTEND=1 ;;
    amd64|linux-amd64|linux) TARGETS+=("amd64") ;;
    jetson|arm64|orin|jetson-orin-nx) TARGETS+=("jetson") ;;
    -h|--help)
      sed -n '2,12p' "$0"
      exit 0
      ;;
    *)
      echo "unknown argument: $arg" >&2
      exit 1
      ;;
  esac
done

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  TARGETS=("amd64" "jetson")
fi

VERSION="$(git -C "$REPO_ROOT" describe --tags --always 2>/dev/null || true)"
VERSION="${VERSION:-dev}"
LDFLAGS="-X main.version=${VERSION}"

if [[ "$SKIP_FRONTEND" -eq 0 && -f "$REPO_ROOT/frontend/package.json" ]]; then
  if command -v npm >/dev/null 2>&1; then
    echo "Building frontend -> embedui/dist"
    (cd "$REPO_ROOT/frontend" && npm run build)
  else
    echo "npm not found; skipping frontend rebuild (using existing embedui/dist)"
  fi
fi

write_helpers() {
  local out_dir="$1"
  cat >"$out_dir/run-librarian.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
DB="$(pwd)/implcache.db"
echo "Starting Librarian on http://127.0.0.1:8080/"
echo "Database: $DB"
exec ./implcache-mcp -db "$DB" -http :8080 \
  -enable-librarian -enable-http-mutations -mode admin
EOF
  cat >"$out_dir/run-librarian-lan.sh" <<'EOF'
#!/usr/bin/env bash
# Bind Librarian + MCP Streamable HTTP on all interfaces for LAN clients
# (Cursor mcp.json "url": "http://<host-ip>:8080/mcp"). See docs/REMOTE.md.
set -euo pipefail
cd "$(dirname "$0")"
DB="$(pwd)/implcache.db"
HOST_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo "Starting Librarian + MCP on http://0.0.0.0:8080/ (-allow-remote-http)"
if [[ -n "${HOST_IP:-}" ]]; then
  echo "  UI:  http://${HOST_IP}:8080/"
  echo "  MCP: http://${HOST_IP}:8080/mcp"
fi
echo "Database: $DB"
exec ./implcache-mcp -db "$DB" \
  -http 0.0.0.0:8080 \
  -allow-remote-http \
  -enable-librarian \
  -enable-http-mutations \
  -mode admin
EOF
  cat >"$out_dir/run-agent.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
DB="$(pwd)/implcache.db"
echo "Starting ImplCache MCP (agent mode, stdio) with DB:"
echo "$DB"
exec ./implcache-mcp -db "$DB" -mode agent
EOF
  chmod +x "$out_dir/run-librarian.sh" "$out_dir/run-librarian-lan.sh" "$out_dir/run-agent.sh"
}

pack_one() {
  local goarch="$1"
  local out_name="$2"
  local label="$3"
  local out_dir="$REPO_ROOT/$out_name"

  echo ""
  echo "== Packing $label (linux/$goarch) -> $out_name/ =="
  mkdir -p "$out_dir"

  # End-user docs/README live under dist/; copy the package shell, then replace binaries.
  if [[ -d "$REPO_ROOT/dist/docs" ]]; then
    rm -rf "$out_dir/docs"
    cp -a "$REPO_ROOT/dist/docs" "$out_dir/docs"
  fi
  if [[ -f "$REPO_ROOT/dist/README.md" ]]; then
    cp -f "$REPO_ROOT/dist/README.md" "$out_dir/README.md"
  fi
  cp -f "$REPO_ROOT/LICENSE" "$out_dir/LICENSE"
  cp -f "$REPO_ROOT/NOTICE" "$out_dir/NOTICE"

  echo "Building implcache-mcp ($VERSION) linux/$goarch"
  (cd "$REPO_ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" \
    go build -ldflags "$LDFLAGS" -o "$out_dir/implcache-mcp" .)
  echo "Building ingestcli linux/$goarch"
  (cd "$REPO_ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" \
    go build -o "$out_dir/ingestcli" ./cmd/ingestcli)
  chmod +x "$out_dir/implcache-mcp" "$out_dir/ingestcli"

  echo "Creating sanitized empty database -> $out_dir/implcache.db"
  (cd "$REPO_ROOT" && go run ./cmd/mkemptydb -o "$out_dir/implcache.db")

  printf '%s' "$VERSION" >"$out_dir/VERSION"
  write_helpers "$out_dir"

  echo "Packed $out_name/:"
  # Portable listing (avoid GNU find -printf; breaks under some Windows bash envs).
  (cd "$out_dir" && ls -1) | while IFS= read -r f; do
    [[ -f "$out_dir/$f" ]] && echo "  $f"
  done
  if [[ -d "$out_dir/docs" ]]; then
    doc_count="$(find "$out_dir/docs" -type f 2>/dev/null | wc -l | tr -d ' ')"
    echo "  docs/  (${doc_count:-?} files)"
  fi
}

for t in "${TARGETS[@]}"; do
  case "$t" in
    amd64)  pack_one amd64 "dist-linux-amd64" "Linux amd64 (WSL)" ;;
    jetson) pack_one arm64 "dist-jetson-orin-nx" "Jetson Orin NX" ;;
  esac
done

echo ""
echo "Done. Ship folders as needed:"
for t in "${TARGETS[@]}"; do
  case "$t" in
    amd64)  echo "  $REPO_ROOT/dist-linux-amd64" ;;
    jetson) echo "  $REPO_ROOT/dist-jetson-orin-nx" ;;
  esac
done
