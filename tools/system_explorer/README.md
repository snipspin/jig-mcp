# system_explorer

Advanced file system search and manipulation tool with path sandboxing. Provides safe file operations within configured base directories.

## Overview

`system_explorer` provides controlled file system access with built-in security:

- **Path sandboxing** - Restricts access to allowed base directories
- **Multiple operations** - read, write, list, search, log tailing
- **Path traversal prevention** - Canonicalizes paths to block `../` attacks
- **Cross-platform** - Bash on Linux/macOS, PowerShell on Windows

### Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `read_file` | Read file contents | `path` |
| `write_file` | Write content to file | `path`, `content` |
| `list_dir` | List directory contents | `path` |
| `search_files` | Find files by pattern | `root`, `pattern` |
| `read_log` | Tail log file (like `tail -n`) | `path`, `lines` |

### Use Cases

- Read configuration files for debugging
- Write output files from workflows
- Browse directory structures
- Find files by name pattern
- Monitor log files in real-time

### Security Model

The tool uses path sandboxing to prevent unauthorized access:

1. **Allowed Base Directories** (configured in script):
   - Current working directory (`$(pwd)`)
   - User home directory (`$HOME`)
   - Temporary directory (`/tmp`)

2. **Path Canonicalization**: Resolves `..` and symlinks to prevent traversal

3. **Validation**: Every path is checked against allowed directories before access

## Configuration

No environment variables required.

### Customizing Allowed Directories

Edit `tools/system_explorer/scripts/explorer.sh` to modify `ALLOWED_BASE_DIRS`:

```bash
ALLOWED_BASE_DIRS=(
    "$(pwd)"
    "$HOME"
    "/tmp"
    "/opt/jig-mcp/data"  # Add custom directory
)
```

## Usage

### Read a File

```json
{
  "name": "system_explorer",
  "arguments": {
    "operation": "read_file",
    "path": "/home/user/config.yaml"
  }
}
```

### Write a File

```json
{
  "name": "system_explorer",
  "arguments": {
    "operation": "write_file",
    "path": "/tmp/output.txt",
    "content": "Hello, World!"
  }
}
```

### List Directory Contents

```json
{
  "name": "system_explorer",
  "arguments": {
    "operation": "list_dir",
    "path": "/home/user/projects"
  }
}
```

### Search for Files

```json
{
  "name": "system_explorer",
  "arguments": {
    "operation": "search_files",
    "root": "/home/user",
    "pattern": "*.log"
  }
}
```

### Read Log File (Tail)

```json
{
  "name": "system_explorer",
  "arguments": {
    "operation": "read_log",
    "path": "/var/log/app.log",
    "lines": 50
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `operation` | string | Operation: `read_file`, `write_file`, `list_dir`, `search_files`, `read_log` | Yes |
| `path` | string | Target file or directory path | Yes (except search_files) |
| `content` | string | Content to write | Yes (write_file only) |
| `pattern` | string | Glob/regex pattern for search | Yes (search_files only) |
| `root` | string | Root directory for search | Yes (search_files only) |
| `lines` | integer | Number of lines to read | No (default: 10) |

## Response Format

### Read File Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "# Configuration file\nkey: value\nport: 8080"
    }
  ]
}
```

### Write File Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Successfully wrote to /tmp/output.txt"
    }
  ]
}
```

### List Directory Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "[{\"name\":\"docs\",\"isDir\":true,\"size\":null},{\"name\":\"README.md\",\"isDir\":false,\"size\":1234}]"
    }
  ]
}
```

### Search Files Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "[\"/home/user/app.log\",\"/home/user/debug.log\",\"/home/user/logs/error.log\"]"
    }
  ]
}
```

### Read Log Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "2024-01-15 10:30:00 INFO Application started\n2024-01-15 10:30:01 INFO Connected to database\n2024-01-15 10:30:02 DEBUG Processing request #1234"
    }
  ]
}
```

### Error Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Access denied: Path '/etc/passwd' is not within allowed base directories."
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: Read Configuration File

**Request:**
```json
{
  "name": "system_explorer",
  "arguments": {
    "operation": "read_file",
    "path": "/home/user/myapp/config.yaml"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "app:\n  name: MyApp\n  version: 1.0.0\n  port: 8080\ndatabase:\n  host: localhost\n  port: 5432"
    }
  ]
}
```

### Example 2: Write Output File

**Request:**
```json
{
  "name": "system_explorer",
  "arguments": {
    "operation": "write_file",
    "path": "/tmp/report.txt",
    "content": "Build completed successfully at 2024-01-15 10:30:00"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Successfully wrote to /tmp/report.txt"
    }
  ]
}
```

### Example 3: List Project Directory

**Request:**
```json
{
  "name": "system_explorer",
  "arguments": {
    "operation": "list_dir",
    "path": "/home/user/projects/myapp"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "[{\"name\":\"src\",\"isDir\":true,\"size\":null},{\"name\":\"tests\",\"isDir\":true,\"size\":null},{\"name\":\"go.mod\",\"isDir\":false,\"size\":156},{\"name\":\"README.md\",\"isDir\":false,\"size\":2048}]"
    }
  ]
}
```

### Example 4: Find All Log Files

**Request:**
```json
{
  "name": "system_explorer",
  "arguments": {
    "operation": "search_files",
    "root": "/home/user",
    "pattern": "*.log"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "[\"/home/user/app.log\",\"/home/user/debug.log\",\"/home/user/projects/myapp/test.log\"]"
    }
  ]
}
```

### Example 5: Monitor Application Log

**Request:**
```json
{
  "name": "system_explorer",
  "arguments": {
    "operation": "read_log",
    "path": "/home/user/myapp/app.log",
    "lines": 20
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "2024-01-15 10:29:45 INFO Starting application\n2024-01-15 10:29:46 INFO Loading configuration\n2024-01-15 10:29:47 INFO Database connection established\n2024-01-15 10:29:48 INFO Server listening on port 8080\n2024-01-15 10:30:00 DEBUG Received request GET /api/status"
    }
  ]
}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Access denied: Path not within allowed base directories" | Path outside sandbox | Move file to allowed directory or add directory to `ALLOWED_BASE_DIRS` |
| "File not found" | File doesn't exist or wrong path | Verify path exists, check permissions |
| "Directory not found" | Directory doesn't exist | Verify path, create directory if needed |
| "Error writing file" | Permission denied or disk full | Check directory permissions, verify disk space |
| "Pattern is required for search_files" | Missing pattern parameter | Provide `pattern` in arguments |
| "Unknown operation" | Invalid operation name | Use valid operation: read_file, write_file, list_dir, search_files, read_log |

## Security Notes

1. **Path Sandboxing**: All paths are validated against `ALLOWED_BASE_DIRS`. This prevents access to sensitive system files like `/etc/passwd`, `/etc/shadow`, etc.

2. **Path Traversal Prevention**: The tool canonicalizes paths using `realpath` or `readlink -f`, which resolves `../` sequences and symlinks. Attempting to access `/home/user/../../../etc/passwd` will be blocked.

3. **Write Operations**: Write access is subject to the same path restrictions. Ensure the target directory exists and is writable.

4. **Information Disclosure**: The tool can reveal file contents and directory structures within allowed directories. Consider this when configuring `ALLOWED_BASE_DIRS`.

5. **Audit Logging**: All file operations are logged in the audit log with operation type, path, and result.

6. **Customizing Allowed Directories**: When adding directories to `ALLOWED_BASE_DIRS`:
   - Use absolute paths
   - Avoid sensitive directories (e.g., `/etc`, `/root`)
   - Consider using subdirectories (e.g., `/opt/jig-mcp/data` instead of `/opt`)

## Implementation Details

### Path Validation

1. Converts relative paths to absolute
2. Uses `realpath -m` or `readlink -f` to canonicalize
3. Checks if path starts with any allowed base directory
4. Rejects paths that don't match

### Operations

- **read_file**: Uses `cat` to read file contents
- **write_file**: Uses `echo -n` to write content (preserves exact content)
- **list_dir**: Uses `ls` and formats as JSON array
- **search_files**: Uses `find -name` with glob patterns
- **read_log**: Uses `tail -n` to read last N lines

## Files

```
tools/system_explorer/
├── manifest.yaml           # Tool configuration
├── README.md               # This documentation
└── scripts/
    ├── explorer.sh         # Bash implementation (Linux/macOS)
    └── explorer.ps1        # PowerShell implementation (Windows)
```

## See Also

- [system_info](../system_info/README.md) - System information tool
- [service_health](../service_health/README.md) - Health check tool
- [Tool Security Guidelines](../../docs/SECURITY.md) - Security best practices
- [Path Traversal Attacks](https://owasp.org/www-community/attacks/Path_Traversal) - OWASP reference
