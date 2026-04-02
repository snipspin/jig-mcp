# scripts/sys_info.ps1
# Simple system info for Windows

if ($args -contains "--mcp-metadata") {
    $metadata = @{
        name = "system_info"
        description = "Provides basic OS and hardware information"
        inputSchema = @{
            type = "object"
            properties = @{}
        }
    }
    Write-Output ($metadata | ConvertTo-Json -Compress -Depth 5)
    exit 0
}

try {
    $info = @{
        hostname = $env:COMPUTERNAME
        os = (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion").ProductName
        powershell_version = $PSVersionTable.PSVersion.ToString()
        uptime = (New-TimeSpan -Start (Get-CimInstance Win32_OperatingSystem).LastBootUpTime -End (Get-Date)).ToString()
        memory_gb = [math]::round((Get-CimInstance Win32_OperatingSystem).TotalVisibleMemorySize / 1MB, 2)
    }
    
    $result = @{
        content = @(
            @{
                type = "text"
                text = $info | ConvertTo-Json
            }
        )
    }
    
    Write-Output ($result | ConvertTo-Json -Compress)
} catch {
    $error_result = @{
        content = @(
            @{
                type = "text"
                text = "Error: $($_.Exception.Message)"
            }
        )
        isError = $true
    }
    Write-Output ($error_result | ConvertTo-Json -Compress)
    exit 1
}
