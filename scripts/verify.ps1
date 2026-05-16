param(
    [switch]$Docker,
    [switch]$Release
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
$previousTag = $env:ORIVIS_IMAGE_TAG

try {
    go test ./...
    golangci-lint run ./...

    if ($Docker) {
        $env:ORIVIS_IMAGE_TAG = "verify"
        go tool bu1ld --no-cache build docker
    }

    if ($Release) {
        if (-not (Get-Command goreleaser -ErrorAction SilentlyContinue)) {
            throw "goreleaser is required for -Release. Install it from https://goreleaser.com/install/."
        }

        goreleaser check
    }
}
finally {
    $env:ORIVIS_IMAGE_TAG = $previousTag
    Pop-Location
}
