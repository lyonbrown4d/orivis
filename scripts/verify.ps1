param(
    [switch]$Docker,
    [switch]$Release
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root

try {
    go test ./...
    golangci-lint run ./...

    if ($Docker) {
        docker build --build-arg APP=orivis-server -t orivis-server:verify .
        docker build --build-arg APP=orivis-agent -t orivis-agent:verify .
    }

    if ($Release) {
        if (-not (Get-Command goreleaser -ErrorAction SilentlyContinue)) {
            throw "goreleaser is required for -Release. Install it from https://goreleaser.com/install/."
        }

        goreleaser check
    }
}
finally {
    Pop-Location
}
