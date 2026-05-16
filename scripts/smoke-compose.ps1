param(
    [string]$Tag = "local-smoke",
    [string]$ProjectName = "orivis-smoke",
    [int]$HostPort = 18080,
    [int]$DurationSeconds = 45,
    [switch]$SkipBuild,
    [switch]$KeepRunning
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $root "deployments\docker-compose\compose.yml"
$serverEnvExample = Join-Path $root "deployments\docker-compose\server.env.example"
$agentEnvExample = Join-Path $root "deployments\docker-compose\agent.env.example"
$serverEnv = Join-Path $root "deployments\docker-compose\server.env"
$agentEnv = Join-Path $root "deployments\docker-compose\agent.env"

function Invoke-CheckedCommand {
    param(
        [string]$FilePath,
        [string[]]$ArgumentList
    )

    & $FilePath @ArgumentList
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code ${LASTEXITCODE}: $FilePath $($ArgumentList -join ' ')"
    }
}

Copy-Item -Force $serverEnvExample $serverEnv
Copy-Item -Force $agentEnvExample $agentEnv

$previousTag = $env:ORIVIS_IMAGE_TAG
$previousPort = $env:ORIVIS_HTTP_PORT

Push-Location $root
try {
    if ($SkipBuild) {
        Invoke-CheckedCommand docker @("image", "inspect", "--format", "{{.Id}}", "ghcr.io/lyonbrown4d/orivis-server:$Tag")
        Invoke-CheckedCommand docker @("image", "inspect", "--format", "{{.Id}}", "ghcr.io/lyonbrown4d/orivis-agent:$Tag")
    }
    else {
        Invoke-CheckedCommand docker @("build", "--build-arg", "APP=orivis-server", "-t", "ghcr.io/lyonbrown4d/orivis-server:$Tag", ".")
        Invoke-CheckedCommand docker @("build", "--build-arg", "APP=orivis-agent", "-t", "ghcr.io/lyonbrown4d/orivis-agent:$Tag", ".")
    }

    $env:ORIVIS_IMAGE_TAG = $Tag
    $env:ORIVIS_HTTP_PORT = "$HostPort"
    $baseURL = "http://127.0.0.1:$HostPort"

    Invoke-CheckedCommand docker @("compose", "-p", $ProjectName, "-f", $composeFile, "down", "--remove-orphans", "-v")
    Invoke-CheckedCommand docker @("compose", "-p", $ProjectName, "-f", $composeFile, "up", "-d", "--remove-orphans")

    $deadline = (Get-Date).AddSeconds($DurationSeconds)
    do {
        Start-Sleep -Seconds 3
        try {
            Invoke-WebRequest -UseBasicParsing "$baseURL/healthz" | Out-Null
            break
        }
        catch {
            if ((Get-Date) -ge $deadline) {
                throw
            }
        }
    } while ((Get-Date) -lt $deadline)

    Start-Sleep -Seconds $DurationSeconds

    $basicAuth = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("admin:change-me"))
    $dashboard = Invoke-WebRequest -UseBasicParsing "$baseURL/" -Headers @{
        Authorization = "Basic $basicAuth"
    }
    foreach ($expected in @("server-health", "redis", "postgres")) {
        if ($dashboard.Content -notmatch [regex]::Escape($expected)) {
            throw "Expected monitor '$expected' was not found on the dashboard."
        }
    }

    Write-Host "Docker Compose smoke run completed with image tag '$Tag'."
    Write-Host "Dashboard: $baseURL/"

    if (-not $KeepRunning) {
        Invoke-CheckedCommand docker @("compose", "-p", $ProjectName, "-f", $composeFile, "down", "-v")
    }
}
finally {
    $env:ORIVIS_IMAGE_TAG = $previousTag
    $env:ORIVIS_HTTP_PORT = $previousPort
    Pop-Location
}
