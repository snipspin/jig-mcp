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

$action    = $args_obj.action
$container = $args_obj.container
$host_addr = $args_obj.host
$tail_lines = if ($args_obj.tail) { [int]$args_obj.tail } else { 100 }
$show_all  = if ($args_obj.all) { $args_obj.all } else { $false }

# Build docker command base args
$docker_args = @()
if ($host_addr) { $docker_args += @("-H", $host_addr) }

function Invoke-Docker {
    param([string[]]$Args)
    $all_args = $docker_args + $Args
    $output = & docker @all_args 2>&1
    if ($LASTEXITCODE -ne 0) { Send-Response "Docker error: $output" $true }
    return ($output | Out-String).Trim()
}

function Require-Container {
    if (-not $container) { Send-Response "Error: 'container' parameter is required for action '$action'" $true }
}

switch ($action) {
    "list" {
        $list_args = @("ps", "--format", "table {{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}")
        if ($show_all) { $list_args = @("ps", "-a", "--format", "table {{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}") }
        $result = Invoke-Docker $list_args
        Send-Response $result
    }
    "start" {
        Require-Container
        Invoke-Docker @("start", $container) | Out-Null
        Send-Response "Container '$container' started successfully."
    }
    "stop" {
        Require-Container
        Invoke-Docker @("stop", $container) | Out-Null
        Send-Response "Container '$container' stopped successfully."
    }
    "restart" {
        Require-Container
        Invoke-Docker @("restart", $container) | Out-Null
        Send-Response "Container '$container' restarted successfully."
    }
    "logs" {
        Require-Container
        $result = Invoke-Docker @("logs", "--tail", "$tail_lines", $container)
        Send-Response $result
    }
    "inspect" {
        Require-Container
        $result = Invoke-Docker @("inspect", $container)
        Send-Response $result
    }
    "remove" {
        Require-Container
        Invoke-Docker @("rm", "-f", $container) | Out-Null
        Send-Response "Container '$container' removed successfully."
    }
    default {
        Send-Response "Unknown action: '$action'. Valid actions: list, start, stop, restart, logs, inspect, remove" $true
    }
}
