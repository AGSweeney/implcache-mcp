#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
DB="$(pwd)/implcache.db"
echo "Starting ImplCache MCP (agent mode, stdio) with DB:"
echo "$DB"
exec ./implcache-mcp -db "$DB" -mode agent
