#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
DB="$(pwd)/implcache.db"
echo "Starting Librarian on http://127.0.0.1:8080/"
echo "Database: $DB"
exec ./implcache-mcp -db "$DB" -http :8080 \
  -enable-librarian -enable-http-mutations -mode admin
