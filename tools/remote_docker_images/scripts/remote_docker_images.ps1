param(
    [string]$json_args
)

$ErrorActionPreference = "Stop"

# Parse JSON arguments
$args_obj = $json_args | ConvertFrom-Json
$action = $args_obj.action
$image = $args_obj.image
$host_param = $args_obj.host
$key = $args_obj.key
$port = if ($args_obj.port) { $args_obj.port } else { 22 }
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

switch ($action) {
    "list" {
        $flags = "--format table {{.Repository}}`t{{.Tag}}`t{{.ID}}`t{{.Size}}`t{{.CreatedSince}}"
        if ($show_all) {
            $flags = "-a $flags"
        }
        $result = ssh $ssh_opts $host_param "docker images $flags" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error listing images: $result" -is_error "true"
        }
        Send-Response -text $result
    }
    "pull" {
        if (-not $image) {
            Send-Response -text "Error: 'image' parameter is required for 'pull' action." -is_error "true"
        }
        $result = ssh $ssh_opts $host_param "docker pull '$image'" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error pulling image: $result" -is_error "true"
        }
        Send-Response -text "Image '$image' pulled successfully.`n$result"
    }
    "remove" {
        if (-not $image) {
            Send-Response -text "Error: 'image' parameter is required for 'remove' action." -is_error "true"
        }
        $result = ssh $ssh_opts $host_param "docker rmi '$image'" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error removing image: $result" -is_error "true"
        }
        Send-Response -text "Image '$image' removed successfully.`n$result"
    }
    "prune" {
        $flags = "-f"
        if ($show_all) {
            $flags = "-a -f"
        }
        $result = ssh $ssh_opts $host_param "docker image prune $flags" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Send-Response -text "Error pruning images: $result" -is_error "true"
        }
        Send-Response -text "Image prune completed.`n$result"
    }
    default {
        Send-Response -text "Unknown action: '$action'. Valid actions: list, pull, remove, prune" -is_error "true"
    }
}
