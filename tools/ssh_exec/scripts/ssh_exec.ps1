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

$host_addr   = $args_obj.host
$command_str = $args_obj.command
$key         = $args_obj.key
$port        = if ($args_obj.port) { [int]$args_obj.port } else { 22 }

if (-not $host_addr) { Send-Response "Error: 'host' parameter is required." $true }
if (-not $command_str) { Send-Response "Error: 'command' parameter is required." $true }

# Build SSH arguments
# Windows 10+ ships with OpenSSH client
$ssh_args = @(
    "-o", "StrictHostKeyChecking=accept-new",
    "-o", "ConnectTimeout=10",
    "-o", "BatchMode=yes",
    "-p", "$port"
)

if ($key) {
    if (-not (Test-Path $key)) { Send-Response "Error: SSH key file not found: $key" $true }
    $ssh_args += @("-i", $key)
}

$ssh_args += @($host_addr, $command_str)

try {
    $output = & ssh @ssh_args 2>&1
    if ($LASTEXITCODE -ne 0) {
        Send-Response "Command failed (exit code $LASTEXITCODE) on ${host_addr}:`n$($output | Out-String)" $true
    }
    Send-Response "Output from ${host_addr}:`n$($output | Out-String)"
}
catch {
    Send-Response "SSH connection failed: $_" $true
}
