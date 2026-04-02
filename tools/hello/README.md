# hello

Example jig-mcp tool that returns a greeting message. Used for testing and as a template for creating new tools.

## Overview

`hello` is a minimal example tool demonstrating:

- **Basic tool structure** - Simple input/output pattern
- **Default parameters** - Optional `name` parameter with default value
- **JSON response format** - Returns MCP-compliant response
- **Cross-platform support** - Bash on Linux/macOS, PowerShell on Windows

### Use Cases

- Testing jig-mcp installation and configuration
- Verifying MCP client connectivity
- Template for creating new tools
- Demonstrating tool development patterns

## Configuration

No environment variables required. No configuration needed.

## Usage

### Basic Greeting (Default)

```json
{
  "name": "hello",
  "arguments": {}
}
```

### Personalized Greeting

```json
{
  "name": "hello",
  "arguments": {
    "name": "Alice"
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `name` | string | Name to greet (default: "World") | No |

## Response Format

```json
{
  "content": [
    {
      "type": "text",
      "text": "Hello, Alice! This is an example jig-mcp tool."
    }
  ]
}
```

## Examples

### Example 1: Default Greeting

**Request:**
```json
{
  "name": "hello",
  "arguments": {}
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Hello, World! This is an example jig-mcp tool."
    }
  ]
}
```

### Example 2: Named Greeting

**Request:**
```json
{
  "name": "hello",
  "arguments": {
    "name": "Developer"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Hello, Developer! This is an example jig-mcp tool."
    }
  ]
}
```

### Example 3: Testing Tool Invocation

Use `hello` to verify your jig-mcp setup is working:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"hello","arguments":{}}}' | ./jig-mcp
```

Expected output:
```json
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"Hello, World! This is an example jig-mcp tool."}]}}
```

## Creating Your Own Tool

The `hello` tool serves as a template. To create a new tool:

1. **Copy the directory structure:**
   ```bash
   cp -r tools/hello tools/my_new_tool
   ```

2. **Edit `manifest.yaml`:**
   - Change `name` and `description`
   - Define your `inputSchema`
   - Adjust `timeout` if needed

3. **Edit the script:**
   - Modify `hello.sh` to implement your logic
   - Parse JSON arguments from `$1`
   - Output JSON response to stdout

4. **Test your tool:**
   ```bash
   ./jig-mcp
   # In another terminal:
   echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"my_new_tool","arguments":{}}}' | ./jig-mcp
   ```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Tool not found: hello" | Tool not registered | Verify `tools/hello/manifest.yaml` exists |
| "jq: command not found" | Missing jq dependency | Install jq: `apt install jq` or `brew install jq` |
| Empty response | Script execution failed | Check script permissions: `chmod +x scripts/hello.sh` |

## Security Notes

1. **No Side Effects**: This tool only outputs a greeting. It performs no system operations.

2. **Safe Example**: The `hello` tool is safe to enable for any user or testing scenario.

3. **Input Handling**: The `name` parameter is safely extracted using `jq` - no shell injection risk.

## Files

```
tools/hello/
├── manifest.yaml       # Tool configuration
├── README.md           # This documentation
└── scripts/
    ├── hello.sh        # Bash implementation (Linux/macOS)
    └── hello.ps1       # PowerShell implementation (Windows)
```

## See Also

- [Tool Developer Guide](../../docs/TOOL_DEVELOPER_GUIDE.md) - Complete guide for building tools
- [docker_echo](../docker_echo/README.md) - Docker connectivity test tool
- [system_info](../system_info/README.md) - System information tool
