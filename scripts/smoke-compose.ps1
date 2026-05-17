param(
    [string]$Tag = "local-smoke",
    [string]$ProjectName = "orivis-smoke",
    [int]$HostPort = 18080,
    [int]$DurationSeconds = 45,
    [switch]$SkipBuild,
    [switch]$DisableDashboardAuth,
    [switch]$UseAgentHCL,
    [switch]$KeepRunning
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $root "deployments\docker-compose\compose.yml"
$composeHCLFile = Join-Path $root "deployments\docker-compose\compose.hcl.yml"
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
if ($DisableDashboardAuth) {
    $serverEnvContent = Get-Content -Raw $serverEnv
    $serverEnvContent = $serverEnvContent.Replace("ORIVIS_AUTH__DASHBOARD__ENABLED=true", "ORIVIS_AUTH__DASHBOARD__ENABLED=false")
    Set-Content -NoNewline -Path $serverEnv -Value $serverEnvContent
}
if ($UseAgentHCL) {
    Set-Content -NoNewline -Path $agentEnv -Value ""
}

$previousTag = $env:ORIVIS_IMAGE_TAG
$previousPort = $env:ORIVIS_HTTP_PORT

Push-Location $root
try {
    $env:ORIVIS_IMAGE_TAG = $Tag

    if ($SkipBuild) {
        Invoke-CheckedCommand docker @("image", "inspect", "--format", "{{.Id}}", "ghcr.io/lyonbrown4d/orivis-server:$Tag")
        Invoke-CheckedCommand docker @("image", "inspect", "--format", "{{.Id}}", "ghcr.io/lyonbrown4d/orivis-agent:$Tag")
    }
    else {
        Invoke-CheckedCommand go @("tool", "bu1ld", "--no-cache", "build", "docker")
    }

    $env:ORIVIS_HTTP_PORT = "$HostPort"
    $baseURL = "http://127.0.0.1:$HostPort"

    $composeArgs = @("compose", "-p", $ProjectName, "-f", $composeFile)
    if ($UseAgentHCL) {
        $composeArgs += @("-f", $composeHCLFile)
    }

    Invoke-CheckedCommand docker ($composeArgs + @("down", "--remove-orphans", "-v"))
    Invoke-CheckedCommand docker ($composeArgs + @("up", "-d", "--remove-orphans"))

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

    if (-not $DisableDashboardAuth) {
        $session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
        Invoke-WebRequest -UseBasicParsing "$baseURL/login" `
            -Method Post `
            -WebSession $session `
            -ContentType "application/json" `
            -Body '{"username":"admin","password":"change-me"}' | Out-Null
    }

    $snapshot = Invoke-RestMethod -Uri "$baseURL/api/dashboard/snapshot"
    if ($DisableDashboardAuth -and $snapshot.auth_enabled) {
        throw "Expected dashboard auth to be disabled."
    }
    foreach ($expected in @("server-health", "redis", "postgres")) {
        $monitor = $snapshot.monitors | Where-Object { $_.name -eq $expected } | Select-Object -First 1
        if ($null -eq $monitor) {
            throw "Expected monitor '$expected' was not found in dashboard snapshot."
        }
        if ($null -eq $monitor.latest -or $monitor.latest.status -ne "up") {
            throw "Expected monitor '$expected' to be up in dashboard snapshot."
        }
    }

    Write-Host "Docker Compose smoke run completed with image tag '$Tag'."
    Write-Host "Dashboard: $baseURL/"

    if (-not $KeepRunning) {
        Invoke-CheckedCommand docker ($composeArgs + @("down", "-v"))
    }
}
finally {
    $env:ORIVIS_IMAGE_TAG = $previousTag
    $env:ORIVIS_HTTP_PORT = $previousPort
    Pop-Location
}
