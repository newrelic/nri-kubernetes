param(
    [string]$WinVersion = "ltsc2019",
    [string]$BinaryName = "nri-kubernetes",
    [string]$BinDir = "./bin",
    [string]$Commit = (git rev-parse HEAD).Trim(),
    [string]$Tag = "dev",
    [string]$CGO_ENABLED = 0,
    [string]$BuildDate = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
)

if (!(Test-Path -Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir -Force
}

Write-Host "[compile-windows] Building $BindaryName for Windows"

if ($env:CGO_ENABLED) { $GO_ENABLED = $env:CGO_ENABLED}
if ($env:Tag) { $Tag = $env:Tag}
if ($env:WinVersion) { $WinVersion = $env:WinVersion}
if ($env:Commit) { $Commit = $env:Commit}
if ($env:BuildDate) { $BuildDate = $env:BuildDate}

$LdFlags = "-X 'main.integrationVersion=$Tag' -X 'main.gitCommit=$Commit' -X 'main.buildDate=$BuildDate'"

Write-Host "go build -ldflags="$LdFlags" -o "$BinDir/$BinaryName-windows-$WinVersion-amd64.exe" ./cmd/nri-kubernetes"
go build -ldflags="$LdFlags" -o "$BinDir/$BinaryName-windows-$WinVersion-amd64.exe" ./cmd/nri-kubernetes
