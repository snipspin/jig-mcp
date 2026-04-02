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
$image     = $args_obj.image
$host_addr = $args_obj.host
$show_all  = if ($args_obj.all) { $args_obj.all } else { $false }

$docker_base = @()
if ($host_addr) { $docker_base += @("-H", $host_addr) }

function Invoke-Docker {
    param([string[]]$Args)
    $all_args = $docker_base + $Args
    $output = & docker @all_args 2>&1
    if ($LASTEXITCODE -ne 0) { Send-Response "Docker error: $output" $true }
    return ($output | Out-String).Trim()
}

switch ($action) {
    "list" {
        $list_args = @("images", "--format", "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}\t{{.CreatedSince}}")
        if ($show_all) { $list_args = @("images", "-a", "--format", "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}\t{{.CreatedSince}}") }
        $result = Invoke-Docker $list_args
        Send-Response $result
    }
    "pull" {
        if (-not $image) { Send-Response "Error: 'image' parameter is required for 'pull' action." $true }
        $result = Invoke-Docker @("pull", $image)
        Send-Response "Image '$image' pulled successfully.`n$result"
    }
    "remove" {
        if (-not $image) { Send-Response "Error: 'image' parameter is required for 'remove' action." $true }
        $result = Invoke-Docker @("rmi", $image)
        Send-Response "Image '$image' removed successfully.`n$result"
    }
    "prune" {
        $prune_args = @("image", "prune", "-f")
        if ($show_all) { $prune_args += "-a" }
        $result = Invoke-Docker $prune_args
        Send-Response "Image prune completed.`n$result"
    }
    default {
        Send-Response "Unknown action: '$action'. Valid actions: list, pull, remove, prune" $true
    }
}
