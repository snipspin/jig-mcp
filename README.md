# jig-mcp: Universal Tool Gateway for Model Context Protocol

[![CI](https://github.com/snipspin/jig-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/snipspin/jig-mcp/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/snipspin/jig-mcp)](https://goreportcard.com/report/github.com/snipspin/jig-mcp)
[![Go Version](https://img.shields.io/github/go-mod/go-version/snipspin/jig-mcp)](go.mod)
[![Release](https://img.shields.io/github/v/release/snipspin/jig-mcp)](https://github.com/snipspin/jig-mcp/releases)
[![GoDoc](https://godoc.org/github.com/snipspin/jig-mcp?status.svg)](https://godoc.org/github.com/snipspin/jig-mcp)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A lightweight MCP server that exposes your existing scripts and tools to any MCP-compatible AI client. Built for homelab operators who run their own LLMs and want real tool access without rewriting anything.

## What is jig-mcp?

`jig-mcp` is a **tool harness**, not a tool framework. You write tools however you want — Bash, PowerShell, Python, Go, whatever runs on your machine. `jig-mcp` provides the MCP plumbing, securely routes requests, and makes your tools available to any AI agent that speaks MCP.

```
Your Scripts/Binaries  →  jig-mcp (YAML config)  →  Any MCP Client
```

Works with local inference (Ollama, llama.cpp, LocalAI), cloud APIs (if you use them), and any MCP-compatible client — Claude Code, VS Code extensions, Open WebUI, custom agents, or anything else that implements the protocol.

## Why jig-mcp?

| Problem | jig-mcp Solution |
|---------|------------------|
| MCP server frameworks require rewriting tools in their language | Drop a YAML manifest. Use any language. |
| Want to expose homelab tools to your local LLM | Run locally, connect via stdio or HTTP (SSE). |
| Need audit trails and resource limits for untrusted scripts | Built-in: structured logs, CPU/memory caps, timeouts. |
| Managing tools scattered across systems is tedious | One manifest per tool. One server to manage them all. |
| Single-machine tools, no remote access | Optional SSE transport: expose over HTTP with token auth. |

## Quick Start

### Install

**Option 1: Pre-built binaries (recommended)**

Download the latest release from the [releases page](https://github.com/snipspin/jig-mcp/releases):

```bash
# Linux (amd64)
curl -LO https://github.com/snipspin/jig-mcp/releases/latest/download/jig-mcp-linux-amd64.tar.gz
tar -xzf jig-mcp-linux-amd64.tar.gz
cd jig-mcp-linux-amd64
sudo ./install.sh

# macOS (arm64)
curl -LO https://github.com/snipspin/jig-mcp/releases/latest/download/jig-mcp-darwin-arm64.tar.gz
tar -xzf jig-mcp-darwin-arm64.tar.gz
cd jig-mcp-darwin-arm64
sudo ./install.sh

# Windows (amd64)
# Download the .zip file from releases and extract jig-mcp.exe
```

The install script copies everything to `/usr/local/jig-mcp/` (binary at `bin/jig-mcp`, plus `tools/`, `scripts/`, and `.env`), then creates a symlink in `/usr/local/bin` for PATH convenience.

**Option 2: Build from source**

```bash
git clone https://github.com/snipspin/jig-mcp.git
cd jig-mcp
cp example.env .env  # Configure tools as needed
go build -o jig-mcp ./cmd/jig-mcp/
```

**Option 3: Go install**

```bash
go install github.com/snipspin/jig-mcp/cmd/jig-mcp@latest
```

### Configure a Tool

**Try the example tool first:**

```bash
# The 'hello' tool in tools/hello/ is a working example you can test immediately
./jig-mcp
# In another terminal, test it:
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"hello","arguments":{"name":"Alice"}}}' | ./jig-mcp
```

Create `tools/my_tool/manifest.yaml`:

```yaml
name: my_tool
description: "Runs a custom script"
inputSchema:
  type: object
  properties:
    query:
      type: string
      description: "What to search for"
  required: ["query"]
platforms:
  linux:
    command: "bash"
    args: ["./tools/my_tool/scripts/search.sh"]
  darwin:
    command: "bash"
    args: ["./tools/my_tool/scripts/search.sh"]
  windows:
    command: "powershell"
    args: ["-ExecutionPolicy", "Bypass", "-File", "./tools/my_tool/scripts/search.ps1"]
timeout: "10s"
maxMemoryMB: 256
maxCPUPercent: 50
```

### Run

```bash
# Stdio mode (default — pipe to any MCP client)
./jig-mcp

# HTTP/SSE mode (remote access from other machines on your network)
./jig-mcp -transport sse -port 3001
```

### Connect to Your MCP Client

Point your MCP client at the binary. Example config (varies by client):

```json
{
  "mcpServers": {
    "jig-mcp": {
      "command": "/path/to/jig-mcp"
    }
  }
}
```

Your tools are now available to whatever AI agent you're running.

## Core Features

### Tool-Agnostic

Write tools in any language. `jig-mcp` only cares about:
1. A YAML manifest describing inputs/outputs
2. A script or binary that reads JSON arguments and outputs JSON

### Per-Tool Configuration

Each tool lives in its own folder with a `manifest.yaml`:
- Platform-specific commands (Windows PowerShell vs. Linux Bash vs. macOS)
- Input/output schema
- Resource limits (CPU, memory)
- Execution timeout
- Optional sandbox isolation (Docker)

### Security & Audit

- **Execution timeout:** Prevent runaway scripts
- **Resource limits:** Cap CPU usage and memory per tool
- **Command sanitization:** Reject manifests with shell metacharacters at load time
- **Structured audit log:** Every tool invocation logged as JSONL with inputs, outputs, duration, and status
- **Token authentication:** Optional `JIG_AUTH_TOKEN` for HTTP/SSE access
- **Sandboxing:** Optional Docker isolation for untrusted tools
- **Sensitive field redaction:** Fields marked `"sensitive": true` in tool schemas are redacted in audit logs
- **Audit log permissions:** Log files are created with `0644`. In sensitive environments, use a restrictive `umask` (e.g., `umask 077`) to limit access.

### Audit Log Rotation

The audit log (`logs/audit.jsonl`) is automatically rotated when it exceeds the size limit. When rotation occurs, the current file is renamed to `audit.jsonl.1`, the previous `.1` becomes `.2`, and so on. The oldest file beyond the retention count is deleted.

Configure via environment variables:
- `JIG_LOG_MAX_SIZE_MB` — max file size before rotation (default: 50)
- `JIG_LOG_MAX_FILES` — number of rotated files to keep (default: 3)

If you prefer external rotation (e.g., on a system already running logrotate):

```
/path/to/logs/audit.jsonl {
    weekly
    rotate 4
    compress
    copytruncate
}
```

Set `JIG_LOG_MAX_SIZE_MB` to a very large value to disable built-in rotation when using external tools.

### Dual Transport

**Stdio mode** (default):
```bash
./jig-mcp
```
Connect locally via stdin/stdout. Standard MCP transport — works with any compliant client.

**SSE/HTTP mode**:
```bash
./jig-mcp -transport sse -port 3001
```
Expose tools over HTTP with Server-Sent Events. Useful for:
- Running jig-mcp on a homelab server, connecting from multiple clients
- Integrating with remote agents or web UIs
- Running behind a reverse proxy

### Rich Output

Tools can return:
- **Text responses** (default)
- **Base64 images** (PNG, JPG — auto-validated)
- **File resources** (downloadable attachments)

The server validates and wraps output into MCP-compliant responses automatically.

### Visual Dashboard

Optional real-time dashboard for monitoring tool execution:

```bash
./jig-mcp -dashboard-port 8080
```

Shows active tools, recent audit log entries, and per-tool call metrics. No external dependencies — pure embedded HTML/JS.

## Bundled Tools

jig-mcp ships with pre-configured tools ready to use. Most tools work out of the box — just add Docker, SSH credentials, or API keys as needed.

| Tool | Description | Type | Config Required |
|------|-------------|------|-----------------|
| [`api_bridge`](tools/api_bridge/README.md) | Generic HTTP/REST API client | HTTP | Target URL |
| [`docker_compose`](tools/docker_compose/README.md) | Manage Docker Compose stacks | External | Docker Compose |
| [`docker_containers`](tools/docker_containers/README.md) | Manage Docker containers | External | Docker installed |
| [`docker_echo`](tools/docker_echo/README.md) | Docker connectivity check | External | Docker installed |
| [`docker_images`](tools/docker_images/README.md) | Manage Docker images | External | Docker installed |
| [`hello`](tools/hello/README.md) | Returns a greeting message | External | None |
| [`remote_docker_compose`](tools/remote_docker_compose/README.md) | Remote Docker Compose via SSH | External | SSH + Docker Compose |
| [`remote_docker_containers`](tools/remote_docker_containers/README.md) | Remote Docker management via SSH | External | SSH + Docker |
| [`remote_docker_images`](tools/remote_docker_images/README.md) | Remote Docker images via SSH | External | SSH + Docker |
| [`service_health`](tools/service_health/README.md) | Ping, port, or HTTP health checks | External | None |
| [`ssh_exec`](tools/ssh_exec/README.md) | Execute commands on remote hosts via SSH | External | SSH access |
| [`system_explorer`](tools/system_explorer/README.md) | File system operations with path sandboxing | External | None |
| [`system_info`](tools/system_info/README.md) | OS and hardware information | External | None |
| [`terminal`](tools/terminal/README.md) | Shell command execution with allowlist | Terminal | Command allowlist |
| [`web_search`](tools/web_search/README.md) | Unified web search (SearXNG, Ollama, or any API) | HTTP | `SEARCH_ENDPOINT`, `SEARCH_METHOD` |

### Tool Categories

**HTTP Tools** - Call external APIs without writing scripts:
- `web_search` - Configurable search backend
- `api_bridge` - Generic REST API client

**External Tools** - Run scripts or binaries:
- `docker_*` (containers, images, compose, echo)
- `hello`, `system_info`, `system_explorer`, `service_health`, `ssh_exec`
- `remote_docker_*` (containers, images, compose)

**Terminal Tool** - Execute shell commands with security allowlist:
- `terminal` - Restricted shell access

Each tool includes detailed documentation in its folder. See [`tools/web_search/README.md`](tools/web_search/README.md) for an example of configuration options and usage patterns.

## Creating Tools

For a comprehensive guide covering all tool types, schemas, testing, and distribution, see the [Tool Developer Guide](docs/TOOL_DEVELOPER_GUIDE.md).

### Simple Bash Example

`tools/disk_usage/manifest.yaml`:
```yaml
name: disk_usage
description: "Report disk usage of a directory"
inputSchema:
  type: object
  properties:
    path:
      type: string
      description: "Path to check (e.g., /home, /var)"
  required: ["path"]
platforms:
  linux:
    command: "bash"
    args: ["./tools/disk_usage/scripts/du.sh"]
  darwin:
    command: "bash"
    args: ["./tools/disk_usage/scripts/du.sh"]
```

`tools/disk_usage/scripts/du.sh`:
```bash
#!/bin/bash
PATH_ARG=$(echo "$1" | jq -r '.path')
du -sh "$PATH_ARG" | jq -R '{text: .}'
```

### Self-Describing Scripts

Scripts can advertise their own schema via `--mcp-metadata`. jig-mcp probes the `scripts/` directory at startup and auto-registers any script that responds:

```bash
#!/bin/bash
if [ "$1" = "--mcp-metadata" ]; then
  echo '{"name":"my_tool","description":"Does something","inputSchema":{"type":"object","properties":{}}}'
  exit 0
fi
# ... normal tool logic ...
```

### HTTP API Bridge

Expose REST APIs as MCP tools without writing any scripts:

```yaml
name: weather_api
description: "Fetch weather data"
http:
  url: "http://localhost:8080/api/weather"
  method: "GET"
  headers:
    Authorization: "Bearer $API_TOKEN"
inputSchema:
  type: object
  properties:
    url:
      type: string
timeout: "15s"
```

> **Security note:** The API bridge allows the LLM to specify target URLs.
> On homelab networks, use `allowedURLPrefixes` in the manifest to restrict
> which hosts the bridge can reach and prevent internal service probing (SSRF attacks).
> Example: restrict to external APIs only with `allowedURLPrefixes: ["https://api.example.com"]`.

### Sandboxed Execution

For untrusted tools, enable Docker isolation:

```yaml
name: untrusted_tool
description: "Runs in isolation"
sandbox:
  type: docker
  image: "alpine:latest"
platforms:
  linux:
    command: "python"
    args: ["./scripts/analysis.py"]
```

## Configuration Reference

### Tool Manifest Fields

```yaml
name: string                    # Tool name (required)
description: string             # What it does (required)
inputSchema: object             # JSON Schema for inputs (required)
platforms: map                  # OS-specific commands
timeout: string                 # Max runtime (e.g., "30s", default "30s")
maxMemoryMB: int                # Memory cap in MB (0 = default 512MB)
maxCPUPercent: int              # CPU usage cap (0 = default 90%)
sandbox:                        # Optional isolation
  type: docker                  # "docker" (wasm planned)
  image: "alpine:latest"
http:                           # Optional HTTP bridge (no script needed)
  url: string
  method: string
  headers: map
terminal:                       # Optional shell command wrapper
  enabled: bool                 # Must be explicitly true
  allowlist: [string]           # Command prefixes allowed
```

### Environment Variables

See `example.env` for a copy-paste template.

```bash
# Authentication (SSE/dashboard only — stdio inherits OS permissions)
JIG_AUTH_TOKEN=your-secret              # Single shared token (caller logged as "default")
JIG_AUTH_TOKENS=agent1:tok1,agent2:tok2 # Named tokens — each caller gets its own audit identity
# If both are set, JIG_AUTH_TOKENS takes precedence.

# Server
JIG_TOOL_TIMEOUT=30s            # Default timeout for all tools
JIG_TRANSPORT=stdio             # Transport mode: "stdio" or "sse"
JIG_SSE_PORT=3001               # Port for SSE transport
JIG_MAX_CONCURRENT_TOOLS=8      # Max simultaneous tool executions (default: min(NumCPU, 8))

# Logging
JIG_LOG_LEVEL=info              # Log level: debug, info, warn, error
JIG_LOG_DIR=logs                # Directory for audit logs
JIG_LOG_MAX_SIZE_MB=50          # Max audit log size before rotation (default: 50)
JIG_LOG_MAX_FILES=3             # Number of rotated audit logs to keep (default: 3)
```

Audit log entries include a `"caller"` field identifying who made the request (`"local"` for stdio, token name for SSE).

### Command-Line Flags

```bash
-version             Print version and exit
-transport string    Transport mode: stdio or sse (default "stdio")
-port int            Port for SSE transport (default 3001)
-dashboard-port int  HTTP port for status dashboard (0 = disabled)
```

## Testing

```bash
go build -o jig-mcp ./cmd/jig-mcp/
go test ./...

# Manual test via stdio
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./jig-mcp
```

## Deployment

### Standalone

Build and run directly. Suitable for single-machine setups:

```bash
go build -o jig-mcp ./cmd/jig-mcp/
./jig-mcp -transport sse -port 3001
```

### Docker

A production-ready [Dockerfile](Dockerfile) is included in the repository:

```bash
docker build -t jig-mcp .
docker run -p 3001:3001 \
  -v ./tools:/jig-mcp/tools \
  -v ./scripts:/jig-mcp/scripts \
  -e JIG_AUTH_TOKEN=your-secret \
  jig-mcp -transport sse -port 3001
```

### systemd

`/etc/systemd/system/jig-mcp.service`:
```ini
[Unit]
Description=jig-mcp MCP Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/jig-mcp -transport sse -port 3001
Environment="JIG_AUTH_TOKEN=your-secret-token"
Environment="JIG_LOG_LEVEL=info"
Restart=on-failure
RestartSec=10
User=jig-mcp

[Install]
WantedBy=multi-user.target
```

## Part of the Jig Project

`jig-mcp` is the MCP tool server component of [Jig](https://github.com/snipspin/jig), a self-hosted agent orchestrator for homelab operators. Jig makes small local LLMs (7B-32B parameters) capable of real agentic work — reliable tool calling, persistent memory, and bounded autonomy — all running on your own hardware.

You can use `jig-mcp` standalone with any MCP client, or as part of the full Jig stack.

## License

[MIT](LICENSE)

## Contributing

Issues, PRs, and feedback welcome. This is a community project for homelab operators who want their local AI setups to actually do things.

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on the development process.

## Project Documentation

| Document | Description |
|----------|-------------|
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute, report bugs, and development setup |
| [CHANGELOG.md](CHANGELOG.md) | Version history and changes |
| [CLAUDE.md](CLAUDE.md) | Development documentation for AI assistants |
| [Tool Developer Guide](docs/TOOL_DEVELOPER_GUIDE.md) | Complete guide for building jig-mcp compatible tools |
