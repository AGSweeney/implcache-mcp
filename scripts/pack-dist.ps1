# Pack a self-contained end-user folder into dist/:
#   binaries (implcache-mcp, ingestcli) + docs/ + LICENSE/NOTICE + README
# Docs under dist/docs are maintained in-repo; this script refreshes binaries,
# license files, and screenshot assets from docs/images/librarian.
param(
    [string]$OutDir = (Join-Path $PSScriptRoot "..\dist"),
    [string]$Version = "",
    [switch]$SkipFrontend
)

$ErrorActionPreference = "Stop"
$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$OutDirPath = Join-Path $PSScriptRoot "..\dist"
if (-not (Test-Path $OutDirPath)) {
    New-Item -ItemType Directory -Path $OutDirPath | Out-Null
}
$OutDir = Resolve-Path -Path $OutDir

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

# Ensure embedded Librarian UI is current before go build.
$frontendDir = Join-Path $RepoRoot "frontend"
if (-not $SkipFrontend -and (Test-Path (Join-Path $frontendDir "package.json"))) {
    Write-Host "Building frontend -> embedui/dist"
    Push-Location $frontendDir
    try {
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "frontend build failed ($LASTEXITCODE)" }
    } finally {
        Pop-Location
    }
}

Write-Host "Building implcache-mcp ($Version) -> $serverOut"
Push-Location $RepoRoot
try {
    go build -ldflags $ldflags -o $serverOut .
    if ($LASTEXITCODE -ne 0) { throw "implcache-mcp build failed ($LASTEXITCODE)" }
    go build -o $cliOut ./cmd/ingestcli
    if ($LASTEXITCODE -ne 0) { throw "ingestcli build failed ($LASTEXITCODE)" }

    Copy-Item -Force (Join-Path $RepoRoot "LICENSE") (Join-Path $OutDir "LICENSE")
    Copy-Item -Force (Join-Path $RepoRoot "NOTICE") (Join-Path $OutDir "NOTICE")

    $shotsSrc = Join-Path $RepoRoot "docs\images\librarian"
    $shotsDst = Join-Path $OutDir "docs\images\librarian"
    if (Test-Path $shotsSrc) {
        New-Item -ItemType Directory -Force -Path $shotsDst | Out-Null
        Copy-Item -Force (Join-Path $shotsSrc "*") $shotsDst
        Write-Host "Synced docs/images/librarian -> dist/docs/images/librarian"
    }

    # Never ship a developer corpus. Always generate a fresh empty schema DB.
    $dbOut = Join-Path $OutDir "implcache.db"
    Write-Host "Creating sanitized empty database -> $dbOut"
    go run ./cmd/mkemptydb -o $dbOut
    if ($LASTEXITCODE -ne 0) { throw "mkemptydb failed ($LASTEXITCODE)" }

    $versionFile = Join-Path $OutDir "VERSION"
    Set-Content -Path $versionFile -Value $Version -NoNewline
} finally {
    Pop-Location
}

Write-Host ""
Write-Host "Packed end-user package:"
Get-ChildItem $OutDir -File | ForEach-Object { "  $($_.Name)" }
Write-Host "  docs/  ($((Get-ChildItem (Join-Path $OutDir 'docs') -Recurse -File).Count) files)"
Write-Host ""
Write-Host "Ship the contents of: $OutDir"
Write-Host "No clone or Go/Node toolchain required to run this folder."
