# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```bash
go build -o jig-mcp ./cmd/jig-mcp/
go test ./...
```

Run a single test:
```bash
go test -run TestName ./path/to/package
```

## Architecture Overview

**jig-mcp** is an MCP (Model Context Protocol) server that exposes user-defined scripts/tools to AI clients via JSON-RPC.

### Core Flow
```
MCP Client → JSON-RPC requests → jig-mcp → Tool execution (scripts/binaries/HTTP) → Audit log
```

### Key Components

| Package / File | Purpose |
|----------------|---------|
| `cmd/jig-mcp/main.go` | Entry point, config loading, graceful shutdown, signal handling |
| `common/common.go` | `Tool` interface (`Definition()`, `Handle()`) and `ToolDef` struct |
| `internal/server/request.go` | JSON-RPC request processing, MCP protocol, panic recovery |
| `internal/sse/sse.go` | HTTP/SSE transport with session management and token auth |
| `internal/auth/auth.go` | Token registry and caller identity propagation via context |
| `internal/config/config.go` | YAML manifest loading, script discovery, shell metachar validation |
| `internal/tools/base.go` | BaseTool with shared timeout/response helpers |
| `internal/tools/external.go` | ExternalTool — script/binary execution with sandbox support |
| `internal/tools/http.go` | HTTPTool — REST API bridge with SSRF prevention |
| `internal/tools/terminal.go` | TerminalTool — shell wrapper with command allowlist |
| `internal/tools/registry.go` | Thread-safe tool registry (RWMutex) |
| `internal/audit/audit.go` | JSONL audit logging with rotation and sensitive field redaction |
| `internal/rlimit/` | Platform-specific resource limits (CPU/memory) for Unix and Windows |
| `internal/logging/logging.go` | Structured slog logging setup |
| `internal/dashboard/dashboard.go` | Optional HTTP monitoring dashboard with metrics API |

### Tool Types

Tools are registered from `tools/*/manifest.yaml` or auto-discovered from `scripts/`:

1. **ExternalTool** - Executes scripts/binaries with platform-specific commands
2. **HTTPTool** - Makes HTTP requests (REST API bridge)
3. **TerminalTool** - Shell command wrapper with allowlist

Each tool implements `common.Tool` interface and handles:
- Input validation via JSON Schema
- Resource limits (CPU%, memory MB, timeout)
- Optional Docker sandbox isolation

### Transport Modes

- **stdio** (default): JSON-RPC over stdin/stdout (newline-delimited)
- **sse**: Server-Sent Events over HTTP (`-transport sse -port 3001`)

### Concurrency

- Tool executions limited by semaphore (`JIG_MAX_CONCURRENT_TOOLS`, default: min(NumCPU, 8))
- Tool registry uses RWMutex for thread-safe lookups
- Audit log writes serialized by mutex (Windows-safe)

### Environment Variables

See `example.env` for template. Key vars:
- `JIG_AUTH_TOKEN` / `JIG_AUTH_TOKENS` - SSE/dashboard authentication
- `JIG_TRANSPORT` - "stdio" or "sse"
- `JIG_SSE_PORT` - Port for SSE transport (default: 3001)
- `JIG_CONFIG_DIR` - Override config directory (auto-detected from binary location)
- `JIG_TOOL_TIMEOUT` - Default tool timeout
- `JIG_MAX_CONCURRENT_TOOLS` - Concurrent tool execution limit (default: 8)
- `JIG_LOG_LEVEL` - Log level: debug, info, warn, error
- `JIG_LOG_DIR`, `JIG_LOG_MAX_SIZE_MB`, `JIG_LOG_MAX_FILES` - Audit log config

**.env loading order:**
1. Current working directory
2. Install root (if binary is in `bin/` subdirectory)
3. Binary directory

### Testing Notes

- Test files use `testdata/` for helper binaries (echo, sleep, oom tools)
- Audit log tests use `JIG_LOG_DIR` override for isolation
- Race detector safe: all shared state protected by mutex/semaphore
