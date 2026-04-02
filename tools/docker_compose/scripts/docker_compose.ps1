param(
    [Parameter(Mandatory = $true)]
    [string]$JsonArgs
)

$ErrorActionPreference = "Stop"

function Send-Response {
    param($Text, $IsError = $false)
    $response = @{
        content = @(@{ type = "text"; text = $Text })
    }
    if ($IsError) { $response.isError = $true }
    $response | ConvertTo-Json -Compress -Depth 5 | Write-Host
    if ($IsError) { exit 1 } else { exit 0 }
}

try { $args_obj = $JsonArgs | ConvertFrom-Json } catch { Send-Response "Failed to parse JSON: $_" $true }

$action      = $args_obj.action
$project_dir = $args_obj.project_dir
$host_addr   = $args_obj.host
$service     = $args_obj.service
$tail_lines  = if ($args_obj.tail) { [int]$args_obj.tail } else { 100 }

if (-not $project_dir) { Send-Response "Error: 'project_dir' parameter is required." $true }

# Build docker base args
$docker_base = @()
if ($host_addr) { $docker_base += @("-H", $host_addr) }

function Invoke-Compose {
    param([string[]]$Args)
    $all_args = $docker_base + @("compose", "-f", "$project_dir/docker-compose.yml") + $Args
    $output = & docker @all_args 2>&1
    if ($LASTEXITCODE -ne 0) { Send-Response "Docker Compose error: $output" $true }
    return ($output | Out-String).Trim()
}

$svc_args = @()
if ($service) { $svc_args = @($service) }

switch ($action) {
    "up" {
        $result = Invoke-Compose (@("up", "-d") + $svc_args)
        Send-Response "Stack in '$project_dir' started successfully.`n$result"
    }
    "down" {
        $result = Invoke-Compose @("down")
        Send-Response "Stack in '$project_dir' stopped and removed.`n$result"
    }
    "ps" {
        $result = Invoke-Compose @("ps")
        Send-Response $result
    }
    "logs" {
        $result = Invoke-Compose (@("logs", "--tail", "$tail_lines") + $svc_args)
        Send-Response $result
    }
    "pull" {
        $result = Invoke-Compose (@("pull") + $svc_args)
        Send-Response "Images pulled for stack in '$project_dir'.`n$result"
    }
    "restart" {
        $result = Invoke-Compose (@("restart") + $svc_args)
        Send-Response "Stack in '$project_dir' restarted.`n$result"
    }
    default {
        Send-Response "Unknown action: '$action'. Valid actions: up, down, ps, logs, pull, restart" $true
    }
}
