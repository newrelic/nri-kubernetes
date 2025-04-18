<#
    .SYNOPSIS
        This script builds the binaries of the New Relic Kubernetes integration for Windows.
#>

param(
    [string]$WinVersion = "ltsc2019",
    [string]$BinaryName = "nri-kubernetes",
    [string]$BinDir = "bin",
    [string]$Commit = (git rev-parse HEAD).Trim(),
    [string]$Tag = "dev",
    [string]$CGO_ENABLED = 0,
    [string]$BuildDate = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
)

$RepoRoot = Join-Path $PSScriptRoot "../.." -Resolve

Push-Location $RepoRoot

try {
    if (!(Test-Path -Path $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir -Force
    }

    Write-Host "[compile-windows] Current folder structure:"
    Get-ChildItem -Recurse | ForEach-Object { Write-Host $_.FullName }

    Write-Host "[compile-windows] Building $BindaryName for Windows"

    if ($env:CGO_ENABLED) { $GO_ENABLED = $env:CGO_ENABLED}
    if ($env:TAG) { $Tag = $env:TAG}
    if ($env:WIN_VERSION) { $WinVersion = $env:WIN_VERSION}
    if ($env:COMMIT) { $Commit = $env:COMMIT}
    if ($env:DATE) { $BuildDate = $env:DATE}

    $LdFlags = "-X 'main.integrationVersion=$Tag' -X 'main.gitCommit=$Commit' -X 'main.buildDate=$BuildDate'"

    Write-Host "go build -ldflags="$LdFlags" -o "$BinDir/$BinaryName-windows-$WinVersion-amd64.exe" ./cmd/nri-kubernetes"
    go build -ldflags="$LdFlags" -o "$BinDir/$BinaryName-windows-$WinVersion-amd64.exe" ./cmd/nri-kubernetes

    Write-Host "[compile-windows] Build complete. Output:" 
    Get-ChildItem -Path $BinDir | ForEach-Object { Write-Host $_.FullName }
} finally {
    Pop-Location
}
