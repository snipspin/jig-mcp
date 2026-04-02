# terminal

Execute authorized shell commands with strict allowlisting. Provides safe shell access for predefined commands only.

## Overview

The `terminal` tool wraps shell command execution with security controls:

- **Command allowlisting** - Only pre-approved commands can execute
- **Output size limits** - Prevents memory exhaustion from large outputs
- **Timeout enforcement** - Commands are killed after timeout
- **Platform support** - Bash on Linux/macOS, PowerShell on Windows

### Use Cases

- Git operations (`git status`, `git log`)
- Directory listing (`ls`, `dir`)
- System info (`whoami`, `uname`)
- Build commands (`go version`, `npm run build`)
- Custom automation scripts

### Security Model

The terminal tool is **disabled by default**. To enable:

1. Set `terminal.enabled: true` in manifest
2. Explicitly list every allowed command in `terminal.allowlist`
3. Commands are matched exactly - `git status` allows only that exact command

## Configuration

Configuration is in `manifest.yaml`. No environment variables required.

### Required Settings

```yaml
terminal:
  enabled: true  # Must be explicitly enabled
  allowlist:
    - "echo"
    - "ls"
    - "git status"
```

### Optional Settings

```yaml
terminal:
  # Maximum output size in bytes (default: 102400 = 100KB)
  maxOutputSize: 102400

# Command timeout (default: 30s)
timeout: 30s
```

### Platform-Specific Commands

```yaml
platforms:
  linux:
    command: "bash"
    args: ["./tools/terminal/scripts/terminal.sh"]
  darwin:
    command: "bash"
    args: ["./tools/terminal/scripts/terminal.sh"]
  windows:
    command: "powershell"
    args: ["-ExecutionPolicy", "Bypass", "-File", "./tools/terminal/scripts/terminal.ps1"]
```

## Usage

### Basic Command

```json
{
  "name": "terminal",
  "arguments": {
    "command": "ls -la"
  }
}
```

### Git Operations

```json
{
  "name": "terminal",
  "arguments": {
    "command": "git status"
  }
}
```

### Build Commands

```json
{
  "name": "terminal",
  "arguments": {
    "command": "go version"
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `command` | string | The shell command to execute | Yes |

## Response Format

Success response with command output:

```json
{
  "content": [
    {
      "type": "text",
      "text": "total 48\ndrwxr-xr-x  5 user staff  160 Jan 15 10:30 .\ndrwxr-xr-x  3 user staff   96 Jan 15 10:30 ..\n-rw-r--r--  1 user staff  1234 Jan 15 10:30 README.md"
    }
  ]
}
```

Error response (command not in allowlist):

```json
{
  "content": [
    {
      "type": "text",
      "text": "Command 'rm -rf /' is not in the allowlist"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: Check Git Status

**Request:**
```json
{
  "name": "terminal",
  "arguments": {
    "command": "git status"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "On branch main\nYour branch is up to date with 'origin/main'.\n\nUntracked files:\n  (use \"git add <file>...\" to include in what will be committed)\n\ttools/terminal/README.md\n\nnothing added to commit but untracked files present"
    }
  ]
}
```

### Example 2: List Directory Contents

**Request:**
```json
{
  "name": "terminal",
  "arguments": {
    "command": "ls -la"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "total 48\ndrwxr-xr-x  5 user staff  160 Jan 15 10:30 .\ndrwxr-xr-x  3 user staff   96 Jan 15 10:30 ..\n-rw-r--r--  1 user staff  1234 Jan 15 10:30 README.md"
    }
  ]
}
```

### Example 3: Check Go Version

**Request:**
```json
{
  "name": "terminal",
  "arguments": {
    "command": "go version"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "go version go1.22.0 linux/amd64"
    }
  ]
}
```

## Allowlist Examples

### Minimal Allowlist (Safe Defaults)

```yaml
terminal:
  enabled: true
  allowlist:
    - "echo"
    - "ls"
    - "dir"
    - "whoami"
    - "pwd"
```

### Development Workflow

```yaml
terminal:
  enabled: true
  allowlist:
    # Git operations
    - "git status"
    - "git log --oneline -10"
    - "git diff"
    - "git branch"
    
    # Build tools
    - "go version"
    - "go build"
    - "go test ./..."
    - "npm run build"
    
    # File operations (read-only)
    - "ls -la"
    - "cat"
    - "head"
    - "tail"
```

### CI/CD Integration

```yaml
terminal:
  enabled: true
  allowlist:
    - "docker ps"
    - "docker images"
    - "docker-compose up -d"
    - "docker-compose down"
    - "kubectl get pods"
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Command not in allowlist" | Command not listed in manifest | Add exact command to `terminal.allowlist` |
| "Terminal tool is disabled" | `enabled: false` in manifest | Set `terminal.enabled: true` |
| "Command timed out" | Execution exceeded timeout | Increase `timeout` in manifest |
| "Output truncated" | Output exceeds `maxOutputSize` | Increase limit or redirect output |
| "Permission denied" | Shell can't execute command | Check file permissions or PATH |

## Security Notes

1. **Exact Match Required**: Commands must match exactly. `ls` does not allow `ls -la`. Either list each variant or use a wrapper script.

2. **No Wildcards**: The allowlist doesn't support wildcards. List each command explicitly.

3. **Argument Validation**: Commands with arguments are matched as complete strings. `"git status"` allows only that exact command, not `"git status -s"`.

4. **Shell Injection**: User input is never interpolated into commands. The command string is passed directly to the shell.

5. **Read-Only by Default**: Start with read-only commands (`ls`, `cat`, `git status`) and only add write operations if absolutely necessary.

6. **Audit Logging**: All terminal commands are logged in the audit log. Review logs regularly for unusual patterns.

7. **Windows Considerations**: On Windows, PowerShell execution policy is bypassed for the script. Ensure only trusted scripts are in the tools directory.

## Files

```
tools/terminal/
├── manifest.yaml       # Tool configuration and allowlist
├── README.md           # This documentation
└── scripts/
    ├── terminal.sh     # Bash implementation (Linux/macOS)
    └── terminal.ps1    # PowerShell implementation (Windows)
```

## See Also

- [Tool Developer Guide](../../docs/TOOL_DEVELOPER_GUIDE.md) - Building custom tools
- [Security Best Practices](../../docs/SECURITY.md) - Security guidelines
- [Audit Logging](../../README.md#audit-logging) - Command audit trails
- [External Tool Implementation](../../internal/tools/external.go) - Source code
