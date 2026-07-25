# Default verification for ImplCache MCP (no CGO required).
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

Write-Host "== gofmt =="
$unfmt = gofmt -l .
if ($unfmt) { Write-Host $unfmt; throw "gofmt dirty" }

Write-Host "== go test =="
go test ./...
Write-Host "== go vet =="
go vet ./...
Write-Host "== go build =="
go build -o nul .
go build -o nul ./cmd/...

Write-Host "== optional race =="
function Find-GccCompatible {
  $onPath = Get-Command gcc -ErrorAction SilentlyContinue
  if ($onPath) { return $onPath.Source }
  $candidates = @(
    "C:\Qt\Tools\mingw*\bin\gcc.exe",
    "C:\msys64\mingw64\bin\gcc.exe",
    "C:\msys64\ucrt64\bin\gcc.exe",
    "C:\msys64\clang64\bin\gcc.exe",
    "C:\mingw64\bin\gcc.exe",
    "$env:USERPROFILE\scoop\apps\mingw\current\bin\gcc.exe"
  )
  foreach ($pattern in $candidates) {
    $hit = Get-Item $pattern -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1
    if ($hit) { return $hit.FullName }
  }
  return $null
}
$gcc = Find-GccCompatible
if ($gcc) {
  Write-Host "using $gcc"
  $env:CGO_ENABLED = "1"
  $env:CC = $gcc
  $gccDir = Split-Path $gcc -Parent
  if ($env:Path -notlike "*$gccDir*") {
    $env:Path = "$gccDir;$env:Path"
  }
  go test -race ./...
} else {
  Write-Host "skipping -race (no gcc-compatible compiler found); concurrent smoke tests still run via go test"
  Write-Host "note: MSVC cl.exe is not enough — Go -race needs MinGW/gcc or clang"
}

Write-Host "== eval =="
go run ./cmd/evaltasks -seed-demo | Out-Null
Write-Host "OK"
