# Changelog

All notable changes to jig-mcp will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [0.1.1] - 2026-04-02

Fix release for v0.1.0 packaging issue.

### Fixed

- Release workflow: create `bin/` subdirectory in release archives

## [0.1.0] - 2026-03-31

Initial public release of jig-mcp.

### Added

#### Core Features
- MCP server implementation with full JSON-RPC 2.0 support
- Dual transport modes: stdio (default) and SSE/HTTP
- YAML manifest-based tool configuration (`tools/*/manifest.yaml`)
- Auto-discovery of self-describing scripts via `--mcp-metadata` flag
- Platform-specific execution (Linux, macOS, Windows)

#### Tool Types
- **ExternalTool**: Execute scripts/binaries with JSON arguments via stdin
- **HTTPTool**: REST API bridge with SSRF prevention (`allowedURLPrefixes`)
- **TerminalTool**: Shell command wrapper with configurable allowlist

#### Security Features
- Shell metacharacter validation at manifest load time (prevents injection)
- Token-based authentication for SSE transport (`JIG_AUTH_TOKEN`, `JIG_AUTH_TOKENS`)
- Per-caller identity propagation for audit trails
- Docker sandbox isolation for untrusted tools
- Per-tool resource limits (CPU%, memory MB, timeout)
- Sensitive field redaction in audit logs (schema-driven)

#### Observability
- Structured JSONL audit logging with automatic rotation
- Optional monitoring dashboard with metrics and recent executions
- Configurable log levels via `JIG_LOG_LEVEL`

#### Reliability
- Graceful shutdown with in-flight tool completion (10s timeout)
- Concurrent tool execution semaphore (`JIG_MAX_CONCURRENT_TOOLS`)
- Panic recovery in tool handlers
- Context-based timeouts throughout

#### Example Tools
- `hello`: Cross-platform greeting tool (Bash + PowerShell)
- `system_info`: OS and hardware information reporter
- `system_explorer`: File system operations with path sandboxing
- `api_bridge`: Generic HTTP/REST API client
- `docker_echo`: Docker connectivity check
- `docker_containers`: Manage Docker containers (list, start, stop, logs, inspect)
- `docker_images`: Manage Docker images (list, pull, remove, prune)
- `docker_compose`: Manage Docker Compose stacks (up, down, ps, logs, pull, restart)
- `ssh_exec`: Execute commands on remote hosts via SSH
- `service_health`: Ping, TCP port, or HTTP health checks
- `web_search`: Web search via configurable backend (SearXNG, Ollama, or any API)
- `remote_docker_containers`: Remote Docker container management via SSH
- `remote_docker_images`: Remote Docker image management via SSH
- `remote_docker_compose`: Remote Docker Compose management via SSH
- `terminal`: Shell command execution with configurable allowlist

#### Documentation
- Comprehensive README with quickstart, examples, and deployment guides
- CONTRIBUTING.md with development setup and code style
- CLAUDE.md for AI assistant development guidance
- TOOL_DEVELOPER_GUIDE.md for standalone tool developers

#### CI/CD
- GitHub Actions workflow for CI (build, vet, test, lint)
- Multi-platform release builds (Linux, macOS, Windows / amd64, arm64)
- GitHub issue templates (bug report, feature request)
- Pull request template with test checklist
- CODEOWNERS configuration for review assignment

### Changed

- Consolidated `CallerIdentity` type to `internal/auth` package (removed duplicate from `internal/server`)

### Fixed

- File permissions for shell scripts in tool directories
- `.gitignore` to exclude build artifacts and audit logs
- `Makefile` clean target to remove all generated files

### Technical Details

- Go version: 1.25.5
- Dependencies: `gopkg.in/yaml.v3` (YAML parsing)

[0.1.1]: https://github.com/snipspin/jig-mcp/releases/tag/v0.1.1
[0.1.0]: https://github.com/snipspin/jig-mcp/releases/tag/v0.1.0
