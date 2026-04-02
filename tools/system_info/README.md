# system_info

Provides basic OS and hardware information about the host system. A simple diagnostic tool for quickly identifying system characteristics.

## Overview

`system_info` returns system information including:

- **OS details** - Operating system, version, kernel
- **CPU information** - Architecture, core count
- **Memory** - Total and available RAM
- **Hostname** - System hostname
- **Platform support** - Bash on Linux/macOS, PowerShell on Windows

### Use Cases

- Quick system identification in multi-host environments
- Debugging environment-specific issues
- Inventory collection for homelab documentation
- Verifying tool execution environment
- Basic health checks for monitoring

### What Information Is Returned

The tool returns a concise summary of system information in plain text format, suitable for quick reading or inclusion in diagnostic reports.

## Configuration

No environment variables required. No configuration needed.

## Usage

### Basic System Info

```json
{
  "name": "system_info",
  "arguments": {}
}
```

That's it - no parameters needed. The tool returns all available system information.

### Input Parameters

This tool takes no input parameters.

## Response Format

### Linux/macOS Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Hostname: homelab-server\nOS: Linux 6.5.0-15-generic\nArchitecture: x86_64\nCPU Cores: 8\nTotal Memory: 32768 MB\nAvailable Memory: 24576 MB"
    }
  ]
}
```

### Windows Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Hostname: DESKTOP-PC\nOS: Microsoft Windows 11 Pro\nArchitecture: AMD64\nCPU Cores: 12\nTotal Memory: 32768 MB\nAvailable Memory: 18432 MB"
    }
  ]
}
```

## Examples

### Example 1: Quick System Check

**Request:**
```json
{
  "name": "system_info",
  "arguments": {}
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Hostname: raspberrypi\nOS: Linux 6.1.21-v8+\nArchitecture: aarch64\nCPU Cores: 4\nTotal Memory: 8192 MB\nAvailable Memory: 4096 MB"
    }
  ]
}
```

### Example 2: Multi-Host Inventory

**Request (parallel execution on multiple hosts):**
```json
[
  {"name": "system_info", "arguments": {}},
  {"name": "system_info", "arguments": {}}
]
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Hostname: server1\nOS: Linux 5.15.0-91-generic\nArchitecture: x86_64\nCPU Cores: 16\nTotal Memory: 65536 MB\nAvailable Memory: 32768 MB"
    },
    {
      "type": "text",
      "text": "Hostname: server2\nOS: Linux 6.5.0-15-generic\nArchitecture: x86_64\nCPU Cores: 8\nTotal Memory: 32768 MB\nAvailable Memory: 16384 MB"
    }
  ]
}
```

### Example 3: Environment Debugging

Use `system_info` before running other tools to verify the execution environment:

**Request:**
```json
{
  "name": "system_info",
  "arguments": {}
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Hostname: build-agent\nOS: Linux 5.15.0-1044-aws\nArchitecture: x86_64\nCPU Cores: 4\nTotal Memory: 8192 MB\nAvailable Memory: 2048 MB"
    }
  ]
}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| Empty or partial output | Script execution failed | Check script permissions and PATH |
| "Command not found" | Missing system commands | Ensure `uname`, `free`, `nproc` are in PATH (Linux) |
| PowerShell execution error | Execution policy blocking | Script uses `-ExecutionPolicy Bypass` flag |

## Security Notes

1. **Read-Only Operation**: This tool only reads system information. It cannot modify system state.

2. **Information Disclosure**: The tool reveals system details that could be used for fingerprinting. Consider this when exposing jig-mcp to untrusted networks.

3. **No External Dependencies**: The tool uses only built-in OS commands (`uname`, `nproc`, `free` on Linux; PowerShell cmdlets on Windows).

4. **Audit Logging**: All invocations are logged in the audit log with timestamp and caller identity.

## Implementation Details

### Linux/macOS

Uses standard commands:
- `hostname` - System hostname
- `uname -r` - Kernel version
- `uname -m` - Architecture
- `nproc` - CPU core count
- `free -m` - Memory information

### Windows

Uses PowerShell cmdlets:
- `$env:COMPUTERNAME` - Hostname
- `[Environment]::OSVersion` - OS version
- `$env:PROCESSOR_ARCHITECTURE` - Architecture
- `Get-CimInstance Win32_Processor` - CPU info
- `Get-CimInstance Win32_OperatingSystem` - Memory info

## Files

```
tools/system_info/
├── manifest.yaml       # Tool configuration
├── README.md           # This documentation
└── scripts/
    ├── sys_info.sh     # Bash implementation (Linux/macOS)
    └── sys_info.ps1    # PowerShell implementation (Windows)
```

## See Also

- [service_health](../service_health/README.md) - Health check tool
- [ssh_exec](../ssh_exec/README.md) - Remote command execution
- [hostname(1)](https://man7.org/linux/man-pages/man1/hostname.1.html) - Linux hostname command
- [Get-CimInstance](https://learn.microsoft.com/en-us/powershell/module/cimcmdlets/get-ciminstance) - PowerShell CIM cmdlets
