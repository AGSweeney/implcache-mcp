@echo off
setlocal
cd /d "%~dp0"
set "DB=%~dp0implcache.db"
echo Starting Librarian on http://127.0.0.1:8080/
echo Database: %DB%
"%~dp0implcache-mcp.exe" -db "%DB%" -http :8080 -enable-librarian -enable-http-mutations -mode admin
