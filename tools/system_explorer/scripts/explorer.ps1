param (
    [Parameter(Mandatory = $true)]
    [string]$JsonArgs
)

# --- Configuration ---
$AllowedBaseDirs = @(
    "$PWD",
    "$HOME",
    "$env:TEMP"
)

# --- Utilities ---
function Send-Response {
    param($Result, $IsError = $false)
    $response = @{
        content = @(@{
                type = "text"
                text = if ($Result -is [string]) { $Result } else { $Result | ConvertTo-Json -Depth 5 }
            })
    }
    if ($IsError) { $response.isError = $true }
    $response | ConvertTo-Json -Compress | Write-Host
    if ($IsError) { exit 1 } else { exit 0 }
}

function Get-SafePath {
    param([string]$Path)
    if (-not $Path) { Send-Response "Path argument is required." $true }
    
    $fullPath = [System.IO.Path]::GetFullPath($Path)
    $isAllowed = $false
    foreach ($base in $AllowedBaseDirs) {
        $fullBase = [System.IO.Path]::GetFullPath($base)
        if ($fullPath.StartsWith($fullBase, [System.StringComparison]::OrdinalIgnoreCase)) {
            $isAllowed = $true
            break
        }
    }
    
    if (-not $isAllowed) {
        Send-Response "Access denied: Path '$Path' is not within allowed base directories." $true
    }
    return $fullPath
}

# --- Main ---
try {
    $argsObj = $JsonArgs | ConvertFrom-Json
}
catch {
    Send-Response "Failed to parse JSON arguments: $_" $true
}

$operation = $argsObj.operation

switch ($operation) {
    "read_file" {
        $path = Get-SafePath $argsObj.path
        if (-not (Test-Path $path -PathType Leaf)) { Send-Response "File not found: $path" $true }
        try {
            $content = Get-Content $path -Raw
            Send-Response $content
        }
        catch {
            Send-Response "Error reading file: $_" $true
        }
    }

    "write_file" {
        $path = Get-SafePath $argsObj.path
        $content = $argsObj.content
        try {
            $dir = Split-Path $path
            if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
            $content | Set-Content $path -NoNewline
            Send-Response "Successfully wrote to $path"
        }
        catch {
            Send-Response "Error writing file: $_" $true
        }
    }

    "list_dir" {
        $path = Get-SafePath $argsObj.path
        if (-not (Test-Path $path -PathType Container)) { Send-Response "Directory not found: $path" $true }
        try {
            $items = Get-ChildItem $path | ForEach-Object {
                @{
                    name         = $_.Name
                    isDir        = $_.PSIsContainer
                    size         = if ($_.PSIsContainer) { $null } else { $_.Length }
                    lastModified = $_.LastWriteTime.ToString("yyyy-MM-dd HH:mm:ss")
                }
            }
            Send-Response $items
        }
        catch {
            Send-Response "Error listing directory: $_" $true
        }
    }

    "search_files" {
        $root = Get-SafePath $argsObj.root
        $pattern = $argsObj.pattern
        if (-not $pattern) { Send-Response "Pattern is required for search_files." $true }
        try {
            # Simple glob search via Get-ChildItem
            $results = Get-ChildItem -Path $root -Filter $pattern -Recurse | ForEach-Object { $_.FullName }
            Send-Response $results
        }
        catch {
            Send-Response "Error searching files: $_" $true
        }
    }

    "read_log" {
        $path = Get-SafePath $argsObj.path
        $lines = if ($argsObj.lines) { [int]$argsObj.lines } else { 10 }
        if (-not (Test-Path $path -PathType Leaf)) { Send-Response "Log file not found: $path" $true }
        try {
            $content = Get-Content $path -Tail $lines | Out-String
            Send-Response $content
        }
        catch {
            Send-Response "Error reading log: $_" $true
        }
    }

    Default {
        Send-Response "Unknown operation: $operation" $true
    }
}
