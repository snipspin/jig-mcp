param(
    [string]$json_args
)

$ErrorActionPreference = "Stop"

# Parse JSON arguments
$args_obj = $json_args | ConvertFrom-Json
$action = $args_obj.action
$container = $args_obj.container
$host_param = $args_obj.host
$key = $args_obj.key
$port = if ($args_obj.port) { $args_obj.port } else { 22 }
$tail_lines = if ($args_obj.tail) { $args_obj.tail } else { 100 }
$show_all = if ($args_obj.all) { $args_obj.all } else { $false }

function Send-Response {
    param(
        [string]$text,
        [string]$is_error = "false"
    )
    $escaped = $text -replace '\\', '\\' -replace '"', '\"' -replace "`r`n", '\n' -replace "`n", '\n' -replace "`t", '\t'
    if ($is_error -eq "true") {
        $response = @{
            content = @(@{ type = "text"; text = $escaped })
            isError = $true
        }
        $response | ConvertTo-Json -Compress
        exit 1
    } else {
        $response = @{
            content = @(@{ type = "text"; text = $escaped })
        }
        $response | ConvertTo-Json -Compress
    }
}

if (-not $host_param) {
    Send-Response -text "Error: 'host' parameter is required." -is_error "true"
}

# Build SSH command
$ssh_opts = "-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -o BatchMode=yes -p $port"
if ($key) {
    if (-not (Test-Path $key)) {
        Send-Response -text "Error: SSH key file not found: $key" -is_error "true"
    }
    $ssh_opts = "$ssh_opts -i $key"
}

# Validate container param for actions that need it
function Require-Container {
    if (-not $container) {
        Send-Response -text "Error: 'container' parameter is required for action '$action'" -is_error "true"
    }
}

switch ($action) {
    "list" {
        $flags = "--format table {{.ID}}`t{{.Names}}`t{{.Image}}`t{{.Status}}`t{{.Ports}}"
        if ($show_all) {
            $flags = "-a $flags"
        }
        $result = ssh $ssh_opts $host_param "docker ps $flags" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error listing containers: $result" -is_error "true"
        }
        Send-Response -text $result
    }
    "start" {
        Require-Container
        $result = ssh $ssh_opts $host_param "docker start '$container'" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error starting container: $result" -is_error "true"
        }
        Send-Response -text "Container '$container' started successfully."
    }
    "stop" {
        Require-Container
        $result = ssh $ssh_opts $host_param "docker stop '$container'" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error stopping container: $result" -is_error "true"
        }
        Send-Response -text "Container '$container' stopped successfully."
    }
    "restart" {
        Require-Container
        $result = ssh $ssh_opts $host_param "docker restart '$container'" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error restarting container: $result" -is_error "true"
        }
        Send-Response -text "Container '$container' restarted successfully."
    }
    "logs" {
        Require-Container
        $result = ssh $ssh_opts $host_param "docker logs --tail '$tail_lines' '$container'" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error fetching logs: $result" -is_error "true"
        }
        Send-Response -text $result
    }
    "inspect" {
        Require-Container
        $result = ssh $ssh_opts $host_param "docker inspect '$container'" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error inspecting container: $result" -is_error "true"
        }
        Send-Response -text $result
    }
    "remove" {
        Require-Container
        $result = ssh $ssh_opts $host_param "docker rm -f '$container'" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error removing container: $result" -is_error "true"
        }
        Send-Response -text "Container '$container' removed successfully."
    }
    default {
        Send-Response -text "Unknown action: '$action'. Valid actions: list, start, stop, restart, logs, inspect, remove" -is_error "true"
    }
}
