#!/bin/bash
# scripts/sys_info.sh
# Simple system info for Linux/macOS

if [ "$1" = "--mcp-metadata" ]; then
    cat <<'METADATA'
{"name":"system_info","description":"Provides basic OS and hardware information","inputSchema":{"type":"object","properties":{}}}
METADATA
    exit 0
fi

OS=$(uname -s)
KERNEL=$(uname -r)
ARCH=$(uname -m)
UPTIME=$(uptime)

# Create JSON response matching MCP format
printf '{"content": [{"type": "text", "text": "OS: %s\nKernel: %s\nArchitecture: %s\nUptime: %s"}]}\n' "$OS" "$KERNEL" "$ARCH" "$UPTIME"
