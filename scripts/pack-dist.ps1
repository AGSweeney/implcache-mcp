# Pack end-user binaries into dist/ alongside sanitized docs.
# Docs under dist/docs are maintained in-repo; this script only builds and copies executables + license files.
param(
    [string]$OutDir = (Join-Path $PSScriptRoot "..\dist"),
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$OutDir = Resolve-Path -Path $OutDir -ErrorAction SilentlyContinue
if (-not $OutDir) {
    New-Item -ItemType Directory -Path (Join-Path $PSScriptRoot "..\dist") | Out-Null
    $OutDir = Resolve-Path (Join-Path $PSScriptRoot "..\dist")
}

if (-not $Version) {
    try {
        $Version = (git -C $RepoRoot describe --tags --always 2>$null).Trim()
    } catch {
        $Version = "dev"
    }
    if (-not $Version) { $Version = "dev" }
}

$exeSuffix = ""
if ($IsWindows -or $env:OS -match "Windows") { $exeSuffix = ".exe" }

$serverOut = Join-Path $OutDir "implcache-mcp$exeSuffix"
$cliOut = Join-Path $OutDir "ingestcli$exeSuffix"
$ldflags = "-X main.version=$Version"

Write-Host "Building implcache-mcp ($Version) -> $serverOut"
Push-Location $RepoRoot
try {
    go build -ldflags $ldflags -o $serverOut .
    go build -o $cliOut ./cmd/ingestcli
    Copy-Item -Force (Join-Path $RepoRoot "LICENSE") (Join-Path $OutDir "LICENSE")
    Copy-Item -Force (Join-Path $RepoRoot "NOTICE") (Join-Path $OutDir "NOTICE")
} finally {
    Pop-Location
}

Write-Host "Packed:"
Get-ChildItem $OutDir -File | ForEach-Object { "  $($_.Name)" }
Write-Host "Docs: $(Join-Path $OutDir 'docs')"
Write-Host "Done. Ship the contents of: $OutDir"
