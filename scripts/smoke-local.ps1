param(
    [int]$DurationSeconds = 30,
    [string]$Addr = "127.0.0.1:8080"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$serverLog = Join-Path $root "tmp\smoke-server.log"
$agentLog = Join-Path $root "tmp\smoke-agent.log"
$dbPath = Join-Path $root "tmp\smoke-orivis.db"

New-Item -ItemType Directory -Force (Join-Path $root "tmp") | Out-Null
Remove-Item -Force -ErrorAction SilentlyContinue $serverLog, $agentLog, $dbPath

$serverEnv = @{
    ORIVIS_APP__ENV = "development"
    ORIVIS_HTTP__ADDR = $Addr
    ORIVIS_LOG__LEVEL = "info"
    ORIVIS_DB__DRIVER = "sqlite"
    ORIVIS_DB__DSN = "file:$dbPath"
    ORIVIS_AUTH__AGENT__TOKEN = "smoke-token"
    ORIVIS_RETENTION__ENABLED = "true"
    ORIVIS_RETENTION__RESULTTTL = "168h"
    ORIVIS_RETENTION__CLEANUPINTERVAL = "1h"
}

$agentEnv = @{
    ORIVIS_SERVER__URL = "http://$Addr"
    ORIVIS_AGENT__NAME = "smoke-agent"
    ORIVIS_AGENT__TOKEN = "smoke-token"
    ORIVIS_AGENT__REGION = "local"
    ORIVIS_AGENT__ENVIRONMENTS = "dev"
    ORIVIS_RUNTIME = "host"
    ORIVIS_POLL__INTERVAL = "5s"
    ORIVIS_DISCOVERY__STATIC__ENABLED = "true"
    ORIVIS_DISCOVERY__STATIC__MONITOR__SOURCE_KEY = "static:smoke-health"
    ORIVIS_DISCOVERY__STATIC__MONITOR__NAME = "smoke-health"
    ORIVIS_DISCOVERY__STATIC__MONITOR__TYPE = "http"
    ORIVIS_DISCOVERY__STATIC__MONITOR__TARGET = "http://$Addr/healthz"
    ORIVIS_DISCOVERY__STATIC__MONITOR__ENVIRONMENT = "dev"
    ORIVIS_DISCOVERY__STATIC__MONITOR__ENABLED = "true"
    ORIVIS_DISCOVERY__STATIC__MONITOR__INTERVAL = "5s"
    ORIVIS_DISCOVERY__STATIC__MONITOR__TIMEOUT = "3s"
    ORIVIS_DISCOVERY__STATIC__MONITOR__RETRY_COUNT = "0"
    ORIVIS_DISCOVERY__STATIC__MONITOR__AGGREGATION = "majority_down"
}

function Start-OrivisProcess {
    param(
        [hashtable]$Env,
        [string]$Command,
        [string]$LogPath
    )

    $envBlock = ($Env.GetEnumerator() | ForEach-Object { '$env:{0}="{1}"' -f $_.Key, $_.Value }) -join "; "
    return Start-Process pwsh -PassThru -WindowStyle Hidden -WorkingDirectory $root -RedirectStandardOutput $LogPath -RedirectStandardError $LogPath -ArgumentList "-NoProfile", "-Command", "$envBlock; $Command"
}

$server = $null
$agent = $null

try {
    $server = Start-OrivisProcess -Env $serverEnv -Command "go run ./cmd/orivis-server" -LogPath $serverLog
    Start-Sleep -Seconds 3
    Invoke-WebRequest -UseBasicParsing "http://$Addr/healthz" | Out-Null

    $agent = Start-OrivisProcess -Env $agentEnv -Command "go run ./cmd/orivis-agent" -LogPath $agentLog
    Start-Sleep -Seconds $DurationSeconds

    Invoke-WebRequest -UseBasicParsing "http://$Addr/" | Out-Null
    Write-Host "Smoke run completed. Logs:"
    Write-Host "  $serverLog"
    Write-Host "  $agentLog"
}
finally {
    if ($agent -and !$agent.HasExited) {
        Stop-Process -Id $agent.Id -Force
    }
    if ($server -and !$server.HasExited) {
        Stop-Process -Id $server.Id -Force
    }
}
