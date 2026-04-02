param(
    [string]$json_args
)

$ErrorActionPreference = "Stop"

# Parse JSON arguments
$args_obj = $json_args | ConvertFrom-Json
$action = $args_obj.action
$project_dir = $args_obj.project_dir
$host_param = $args_obj.host
$key = $args_obj.key
$port = if ($args_obj.port) { $args_obj.port } else { 22 }
$service = $args_obj.service
$tail_lines = if ($args_obj.tail) { $args_obj.tail } else { 100 }

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
if (-not $project_dir) {
    Send-Response -text "Error: 'project_dir' parameter is required." -is_error "true"
}

# Build SSH command
$ssh_opts = "-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -o BatchMode=yes -p $port"
if ($key) {
    if (-not (Test-Path $key)) {
        Send-Response -text "Error: SSH key file not found: $key" -is_error "true"
    }
    $ssh_opts = "$ssh_opts -i $key"
}

# Build compose command
$compose_cmd = "docker compose -f '$project_dir/docker-compose.yml'"

# Append service if specified
$svc_arg = ""
if ($service) {
    $svc_arg = $service
}

switch ($action) {
    "up" {
        $result = ssh $ssh_opts $host_param "$compose_cmd up -d $svc_arg" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error starting stack: $result" -is_error "true"
        }
        Send-Response -text "Stack in '$project_dir' started successfully.`n$result"
    }
    "down" {
        $result = ssh $ssh_opts $host_param "$compose_cmd down" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error stopping stack: $result" -is_error "true"
        }
        Send-Response -text "Stack in '$project_dir' stopped and removed.`n$result"
    }
    "ps" {
        $result = ssh $ssh_opts $host_param "$compose_cmd ps" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error listing services: $result" -is_error "true"
        }
        Send-Response -text $result
    }
    "logs" {
        if ($svc_arg) {
            $result = ssh $ssh_opts $host_param "$compose_cmd logs --tail '$tail_lines' $svc_arg" 2>&1
        } else {
            $result = ssh $ssh_opts $host_param "$compose_cmd logs --tail '$tail_lines'" 2>&1
        }
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error fetching logs: $result" -is_error "true"
        }
        Send-Response -text $result
    }
    "pull" {
        $result = ssh $ssh_opts $host_param "$compose_cmd pull $svc_arg" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error pulling images: $result" -is_error "true"
        }
        Send-Response -text "Images pulled for stack in '$project_dir'.`n$result"
    }
    "restart" {
        $result = ssh $ssh_opts $host_param "$compose_cmd restart $svc_arg" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error restarting stack: $result" -is_error "true"
        }
        Send-Response -text "Stack in '$project_dir' restarted.`n$result"
    }
    default {
        Send-Response -text "Unknown action: '$action'. Valid actions: up, down, ps, logs, pull, restart" -is_error "true"
    }
}
