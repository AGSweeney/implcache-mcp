#!/usr/bin/env bash
# Bind Librarian + MCP Streamable HTTP on all interfaces for LAN clients
# (Cursor mcp.json "url": "http://<jetson-ip>:8080/mcp").
# Requires a trusted network; see docs/REMOTE.md.
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
