@echo off
setlocal
cd /d "%~dp0"
set "DB=%~dp0implcache.db"
echo Starting ImplCache MCP (agent mode, stdio) with DB:
echo %DB%
echo Configure Cursor to launch this script, or call implcache-mcp.exe directly with these args.
"%~dp0implcache-mcp.exe" -db "%DB%" -mode agent
