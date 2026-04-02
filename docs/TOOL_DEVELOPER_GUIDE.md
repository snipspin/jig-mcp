# jig-mcp Tool Developer Guide

This document provides complete instructions for developing standalone tools compatible with jig-mcp and other MCP servers that support external tool execution.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Tool Types](#tool-types)
3. [Creating an ExternalTool](#creating-an-externaltool)
4. [Manifest File Specification](#manifest-file-specification)
5. [Script Development](#script-development)
6. [MCP Metadata Protocol](#mcp-metadata-protocol)
7. [Input Schema Design](#input-schema-design)
8. [Output Format Specification](#output-format-specification)
9. [Sandbox Support](#sandbox-support)
10. [Resource Limits](#resource-limits)
11. [Security Considerations](#security-considerations)
12. [Testing Your Tool](#testing-your-tool)
13. [Distribution](#distribution)
14. [Example Tools](#example-tools)

---

## Architecture Overview

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   MCP Client    │────▶│   jig-mcp        │────▶│  Your Tool      │
│   (Claude, etc) │◀────│   (MCP Server)   │◀────│  (script/binary)│
└─────────────────┘     └──────────────────┘     └─────────────────┘
        │                        │                        │
        │   JSON-RPC request     │                        │
        │───────────────────────▶│                        │
        │                        │   Execute with args    │
        │                        │───────────────────────▶│
        │                        │   JSON on stdin        │
        │                        │                        │
        │                        │   JSON/text on stdout  │
        │                        │◀───────────────────────│
        │   CallToolResult       │                        │
        │◀───────────────────────│                        │
```

### Key Concepts

- **Tools are isolated**: Each tool runs as a separate process with no shared state
- **JSON communication**: Arguments passed as JSON via stdin, results returned via stdout
- **No dependencies on jig-mcp internals**: Tools are completely standalone
- **Multiple tool types**: External scripts, HTTP endpoints, terminal commands

---

## Tool Types

jig-mcp supports three tool types:

### 1. ExternalTool (Recommended for standalone tools)

Executes a script or binary with JSON arguments passed via stdin.

**Use when**: You need custom logic, file processing, API integrations, or complex computations.

### 2. HTTPTool

Makes HTTP requests to external APIs.

**Use when**: Wrapping REST APIs, webhooks, or web services.

### 3. TerminalTool

Executes shell commands with an allowlist for safety.

**Use when**: Wrapping system commands like `docker`, `git`, `kubectl`.

---

## Creating an ExternalTool

### Step 1: Create Your Tool Directory

```
my-tool/
├── manifest.yaml
└── my_tool.py  # or .sh, .go binary, etc.
```

### Step 2: Write the manifest.yaml

```yaml
name: my-tool
description: A brief description of what your tool does (shown to users)
inputSchema:
  type: object
  properties:
    param1:
      type: string
      description: Description of param1
    param2:
      type: integer
      description: Description of param2
      default: 10
  required:
    - param1

# Platform-specific execution commands
platforms:
  linux:
    command: python3
    args:
      - /path/to/my_tool.py
  darwin:
    command: python3
    args:
      - /path/to/my_tool.py
  windows:
    command: powershell
    args:
      - -ExecutionPolicy
      - Bypass
      - -File
      - C:\path\to\my_tool.ps1

# Optional: Resource limits
timeout: 60s              # Max execution time
maxMemoryMB: 512          # Max memory usage
maxCPUPercent: 50         # Max CPU usage

# Optional: Sandbox isolation
sandbox:
  type: docker
  image: python:3.11-slim
```

### Step 3: Write Your Script

Your script must:
1. Read JSON from stdin
2. Process the arguments
3. Write result to stdout (JSON or text)

---

## Manifest File Specification

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Tool identifier (lowercase, hyphens allowed) |
| `description` | string | Human-readable description |
| `inputSchema` | object | JSON Schema for input validation |
| `platforms` | object | Platform-specific execution config (unless using http/terminal) |

### Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | string | 30s | Execution timeout (e.g., "60s", "5m") |
| `maxMemoryMB` | integer | unlimited | Memory limit in MB |
| `maxCPUPercent` | integer | unlimited | CPU limit percentage |
| `sandbox` | object | none | Sandbox configuration |

### Platform Configuration

Each platform entry specifies how to execute the tool:

```yaml
platforms:
  linux:
    command: bash
    args:
      - /opt/tools/myscript.sh
  darwin:
    command: bash
    args:
      - /opt/tools/myscript.sh
  windows:
    command: powershell
    args:
      - -File
      - C:\tools\myscript.ps1
```

**Important**:
- Commands cannot contain shell metacharacters (`;`, `|`, `&`, `>`, `<`, `` ` ``, `$(`, `${`)
- Commands cannot contain whitespace (use `args` for parameters)
- Paths should be absolute for reliability

---

## Script Development

### Python Example

```python
#!/usr/bin/env python3
"""
my_tool.py - Example jig-mcp external tool
"""

import sys
import json


def main():
    # Read JSON arguments from stdin
    try:
        input_data = sys.stdin.read()
        args = json.loads(input_data)
    except json.JSONDecodeError as e:
        print(json.dumps({
            "content": [{"type": "text", "text": f"Invalid JSON input: {e}"}],
            "isError": True
        }))
        sys.exit(1)

    # Extract parameters
    param1 = args.get("param1", "")
    param2 = args.get("param2", 10)

    # Your tool logic here
    result = f"Received param1={param1}, param2={param2}"

    # Output result (MCP CallToolResult format)
    output = {
        "content": [{"type": "text", "text": result}]
    }
    print(json.dumps(output))


if __name__ == "__main__":
    main()
```

### Go Example

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

type ToolInput struct {
    Param1 string `json:"param1"`
    Param2 int    `json:"param2,omitempty"`
}

type ContentItem struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

type CallToolResult struct {
    Content []ContentItem `json:"content"`
    IsError bool          `json:"isError,omitempty"`
}

func main() {
    var args ToolInput
    if err := json.NewDecoder(os.Stdin).Decode(&args); err != nil {
        result := CallToolResult{
            Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Input error: %v", err)}},
            IsError: true,
        }
        json.NewEncoder(os.Stdout).Encode(result)
        os.Exit(1)
    }

    // Your tool logic here
    resultText := fmt.Sprintf("Received param1=%s, param2=%d", args.Param1, args.Param2)

    result := CallToolResult{
        Content: []ContentItem{{Type: "text", Text: resultText}},
    }
    json.NewEncoder(os.Stdout).Encode(result)
}
```

### Bash Example

```bash
#!/bin/bash
# my_tool.sh - Example jig-mcp external tool

# Read stdin as JSON
INPUT=$(cat)

# Parse with jq (must be installed)
PARAM1=$(echo "$INPUT" | jq -r '.param1 // ""')
PARAM2=$(echo "$INPUT" | jq -r '.param2 // 10')

# Your tool logic here
RESULT="Received param1=$PARAM1, param2=$PARAM2"

# Output as MCP result
jq -n --arg text "$RESULT" '{
    content: [{type: "text", text: $text}]
}'
```

### PowerShell Example

```powershell
# my_tool.ps1 - Example jig-mcp external tool

# Read JSON from stdin
$inputData = Get-Content -Raw | ConvertFrom-Json

$param1 = $inputData.param1
$param2 = $inputData.param2

# Your tool logic here
$result = "Received param1=$param1, param2=$param2"

# Output as MCP result
$output = @{
    content = @(
        @{
            type = "text"
            text = $result
        }
    )
}

$output | ConvertTo-Json -Depth 10
```

---

## MCP Metadata Protocol

jig-mcp can auto-discover scripts that support `--mcp-metadata`. This allows scripts in the `scripts/` directory to be registered as tools without a manifest file.

### Implementing --mcp-metadata

Add a flag handler to your script that outputs tool metadata:

```python
#!/usr/bin/env python3
import sys
import json

METADATA = {
    "name": "my-tool",
    "description": "A brief description of what this tool does",
    "inputSchema": {
        "type": "object",
        "properties": {
            "param1": {
                "type": "string",
                "description": "Description of param1"
            },
            "param2": {
                "type": "integer",
                "description": "Description of param2",
                "default": 10
            }
        },
        "required": ["param1"]
    }
}

if "--mcp-metadata" in sys.argv:
    print(json.dumps(METADATA))
    sys.exit(0)

# Normal tool execution follows...
```

### Metadata JSON Structure

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Tool identifier |
| `description` | string | Human-readable description |
| `inputSchema` | object | JSON Schema (Draft 7) for input validation |

When a script supports `--mcp-metadata`, jig-mcp will:
1. Execute the script with `--mcp-metadata` at startup
2. Parse the JSON output
3. Automatically register the tool with appropriate platform entries

---

## Input Schema Design

jig-mcp uses JSON Schema (Draft 7) for input validation. Design schemas that are clear and validated.

### Basic Types

```yaml
inputSchema:
  type: object
  properties:
    # String
    name:
      type: string
      description: A person's name

    # Integer
    count:
      type: integer
      description: Number of items
      minimum: 1
      maximum: 1000

    # Boolean
    verbose:
      type: boolean
      description: Enable verbose output
      default: false

    # Number (float)
    threshold:
      type: number
      description: Threshold value
      minimum: 0.0
      maximum: 1.0

    # Array
    tags:
      type: array
      description: List of tags
      items:
        type: string

    # Object
    config:
      type: object
      description: Configuration object
      properties:
        key:
          type: string
        value:
          type: string
```

### Common Patterns

#### File Path Input
```yaml
file_path:
  type: string
  description: Path to the file to process
```

#### URL Input with Validation
```yaml
url:
  type: string
  description: The URL to fetch
  format: uri
  pattern: "^https?://"
```

#### Enum (Fixed Options)
```yaml
format:
  type: string
  description: Output format
  enum:
    - json
    - yaml
    - text
  default: json
```

#### Conditional Required Fields
```yaml
inputSchema:
  type: object
  properties:
    mode:
      type: string
      enum: [file, url]
    file_path:
      type: string
    url:
      type: string
  required:
    - mode
```

---

## Output Format Specification

### MCP CallToolResult Format (Recommended)

Return structured results for best compatibility:

```json
{
  "content": [
    {
      "type": "text",
      "text": "Your result message"
    }
  ]
}
```

### Content Types

#### Text Content
```json
{
  "content": [
    {
      "type": "text",
      "text": "Operation completed successfully"
    }
  ]
}
```

#### Image Content
```json
{
  "content": [
    {
      "type": "image",
      "data": "base64-encoded-image-data-here",
      "mimeType": "image/png"
    }
  ]
}
```

#### Multiple Content Items
```json
{
  "content": [
    {
      "type": "text",
      "text": "Processing complete. Found 3 items:"
    },
    {
      "type": "text",
      "text": "- Item 1\n- Item 2\n- Item 3"
    }
  ]
}
```

### Error Results

Include `isError: true` for errors:

```json
{
  "content": [
    {
      "type": "text",
      "text": "Error description here"
    }
  ],
  "isError": true
}
```

### Simple Text Output (Fallback)

If your script outputs plain text (not JSON), jig-mcp will automatically wrap it:

```
This plain text output will be wrapped in an MCP response
```

Becomes:
```json
{
  "content": [{"type": "text", "text": "This plain text output..."}]
}
```

---

## Sandbox Support

Tools can optionally run in Docker sandboxes for isolation.

### Docker Sandbox Configuration

```yaml
sandbox:
  type: docker
  image: python:3.11-slim
```

### Benefits

- **Isolation**: Tool runs in a separate container
- **Reproducibility**: Consistent environment across systems
- **Security**: Limited access to host system

### Considerations

- Docker must be installed on the host
- Adds startup overhead
- Volume mounts not enabled by default (security)

### Writing Sandbox-Compatible Tools

1. **Self-contained**: Include all dependencies in the image
2. **No host assumptions**: Don't assume host paths or services
3. **Environment variables**: Pass config via env vars, not host files

Example with custom Docker image:

```yaml
sandbox:
  type: docker
  image: myregistry/jig-tool-pdf:latest
```

---

## Resource Limits

Prevent runaway tools from consuming system resources.

### Configuration

```yaml
# Timeout
timeout: 60s

# Memory limit (MB)
maxMemoryMB: 512

# CPU limit (percentage)
maxCPUPercent: 50
```

### Best Practices

1. **Set appropriate timeouts**: Most tools should complete in <30s
2. **Estimate memory**: Add headroom for peak usage
3. **Test limits**: Verify your tool works within configured limits

### Handling Resource Limit Errors

Your tool may be killed if it exceeds limits. Handle gracefully:

```python
import sys

def process_large_file(path):
    # Process in chunks to limit memory
    with open(path, 'rb') as f:
        for chunk in iter(lambda: f.read(8192), b''):
            process(chunk)

    # Check progress and exit early if needed
    if progress > MAX_ALLOWED:
        print(json.dumps({
            "content": [{"type": "text", "text": "Operation stopped: exceeded limits"}],
            "isError": True
        }))
        sys.exit(1)
```

---

## Security Considerations

### For Tool Developers

1. **Validate all inputs**: Never trust input parameters
2. **Avoid shell injection**: Use parameterized commands
3. **Limit file access**: Only access intended files/directories
4. **Sanitize output**: Don't leak sensitive information
5. **Use allowlists**: For URLs, file paths, commands

### Example: Safe Command Execution

```python
# BAD - vulnerable to injection
import os
filename = args.get("filename")
os.system(f"cat {filename}")  # DANGEROUS!

# GOOD - safe subprocess usage
import subprocess
filename = args.get("filename")
# Validate filename first
if not os.path.basename(filename) == filename:
    raise ValueError("Invalid filename")
subprocess.run(["cat", filename], capture_output=True)
```

### Example: URL Validation

```python
from urllib.parse import urlparse
import ipaddress

def is_safe_url(url):
    parsed = urlparse(url)

    # Only allow HTTP/HTTPS
    if parsed.scheme not in ('http', 'https'):
        return False

    # Check for internal IPs
    try:
        ip = ipaddress.ip_address(parsed.hostname)
        if ip.is_private or ip.is_loopback:
            return False
    except ValueError:
        pass  # DNS name, allow

    return True
```

### Manifest Security

jig-mcp validates manifests at load time:
- Shell metacharacters rejected in commands
- Whitespace in command names rejected
- Terminal tools require explicit allowlists

---

## Testing Your Tool

### Manual Testing

Test your script directly before integrating:

```bash
# Test with sample input
echo '{"param1": "test", "param2": 42}' | python3 my_tool.py

# Test metadata endpoint
python3 my_tool.py --mcp-metadata
```

### Integration Testing

1. **Copy to jig-mcp**: Place tool in `tools/` or `scripts/`
2. **Start jig-mcp**: `go run ./cmd/jig-mcp/`
3. **List tools**: Verify your tool appears
4. **Call tool**: Use MCP client to invoke

### Example Test Script

```python
#!/usr/bin/env python3
"""Test harness for jig-mcp tools"""

import subprocess
import json
import sys

def test_tool(script_path, test_inputs):
    """Run tool with test inputs and verify output"""
    for input_data, expected_check in test_inputs:
        result = subprocess.run(
            ["python3", script_path],
            input=json.dumps(input_data),
            capture_output=True,
            text=True
        )

        try:
            output = json.loads(result.stdout)
            if expected_check(output):
                print(f"✓ Test passed")
            else:
                print(f"✗ Test failed: {output}")
        except json.JSONDecodeError:
            print(f"✗ Invalid JSON output: {result.stdout}")

# Example usage
if __name__ == "__main__":
    test_tool("my_tool.py", [
        ({"param1": "hello", "param2": 10}, lambda r: "hello" in str(r)),
        ({"param1": "world"}, lambda r: "world" in str(r)),
    ])
```

---

## Distribution

### Repository Structure

For a standalone tool repository:

```
jig-mcp-tools/
├── README.md                 # Tool catalog with installation instructions
├── tools/
│   ├── pdf-extract/
│   │   ├── manifest.yaml     # Copy this to your jig-mcp/tools/
│   │   └── pdf_extract.py    # Script to place in scripts/
│   ├── telegram-fetch/
│   │   └── manifest.yaml
│   └── ...
└── scripts/                  # Alternative: scripts with --mcp-metadata
    ├── pdf_extract.py
    └── telegram_fetch.py
```

### README Template

```markdown
# jig-mcp Tools

Standalone tools for jig-mcp and compatible MCP servers.

## Installation

1. Copy `tools/<tool-name>/manifest.yaml` to your jig-mcp `tools/<tool-name>/` directory
2. Copy the script to your jig-mcp `scripts/` directory (or update paths in manifest)
3. Restart jig-mcp

## Available Tools

### pdf-extract

Extract text and images from PDF files.

**Input:**
- `file_path` (string): Path to PDF file
- `extract_images` (boolean, optional): Also extract images

**Output:** Text content and optionally image data

**Manifest:** [tools/pdf-extract/manifest.yaml](tools/pdf-extract/manifest.yaml)
```

### Version Compatibility

Document which jig-mcp versions your tools support:

```yaml
# In manifest.yaml or README
metadata:
  min_jig_mcp_version: "0.1.0"
  tested_versions:
    - "0.1.0"
    - "0.2.0"
```

---

## Example Tools

### Example 1: PDF Extract Tool

```yaml
# tools/pdf-extract/manifest.yaml
name: pdf-extract
description: Extract text and optionally images from PDF files
inputSchema:
  type: object
  properties:
    file_path:
      type: string
      description: Path to the PDF file
    extract_images:
      type: boolean
      description: Whether to extract images
      default: false
  required:
    - file_path

platforms:
  linux:
    command: python3
    args:
      - /opt/jig-mcp/scripts/pdf_extract.py
  darwin:
    command: python3
    args:
      - /opt/jig-mcp/scripts/pdf_extract.py
  windows:
    command: powershell
    args:
      - -ExecutionPolicy
      - Bypass
      - -File
      - C:\jig-mcp\scripts\pdf_extract.ps1

timeout: 60s
maxMemoryMB: 256
```

```python
# scripts/pdf_extract.py
#!/usr/bin/env python3
"""Extract text and optionally images from PDF files."""

import sys
import json
import base64

try:
    from pypdf import PdfReader
except ImportError:
    print(json.dumps({
        "content": [{"type": "text", "text": "pypdf not installed. Run: pip install pypdf"}],
        "isError": True
    }))
    sys.exit(1)

METADATA = {
    "name": "pdf-extract",
    "description": "Extract text and optionally images from PDF files",
    "inputSchema": {
        "type": "object",
        "properties": {
            "file_path": {
                "type": "string",
                "description": "Path to the PDF file"
            },
            "extract_images": {
                "type": "boolean",
                "description": "Whether to extract images",
                "default": False
            }
        },
        "required": ["file_path"]
    }
}

if "--mcp-metadata" in sys.argv:
    print(json.dumps(METADATA))
    sys.exit(0)

def main():
    try:
        input_data = sys.stdin.read()
        args = json.loads(input_data)
    except json.JSONDecodeError as e:
        print(json.dumps({
            "content": [{"type": "text", "text": f"Invalid JSON input: {e}"}],
            "isError": True
        }))
        sys.exit(1)

    file_path = args.get("file_path", "")
    extract_images = args.get("extract_images", False)

    if not file_path:
        print(json.dumps({
            "content": [{"type": "text", "text": "Missing required parameter: file_path"}],
            "isError": True
        }))
        sys.exit(1)

    try:
        reader = PdfReader(file_path)
        text_content = []
        images = []

        for i, page in enumerate(reader.pages):
            text_content.append(f"--- Page {i + 1} ---")
            text_content.append(page.extract_text() or "")

            if extract_images:
                # Extract images from page (simplified)
                pass

        result = {
            "content": [
                {"type": "text", "text": "\n".join(text_content)}
            ]
        }

        if images:
            for img_data in images:
                result["content"].append({
                    "type": "image",
                    "data": img_data,
                    "mimeType": "image/png"
                })

        print(json.dumps(result))

    except FileNotFoundError:
        print(json.dumps({
            "content": [{"type": "text", "text": f"File not found: {file_path}"}],
            "isError": True
        }))
    except Exception as e:
        print(json.dumps({
            "content": [{"type": "text", "text": f"Error processing PDF: {e}"}],
            "isError": True
        }))

if __name__ == "__main__":
    main()
```

### Example 2: Telegram Fetch Tool

```yaml
# tools/telegram-fetch/manifest.yaml
name: telegram-fetch
description: Fetch messages from a Telegram channel or chat
inputSchema:
  type: object
  properties:
    chat_id:
      type: string
      description: Channel username (e.g., @channelname) or numeric chat ID
    limit:
      type: integer
      description: Number of messages to fetch
      default: 10
      minimum: 1
      maximum: 100
    offset:
      type: integer
      description: Number of messages to skip
      default: 0
  required:
    - chat_id

platforms:
  linux:
    command: python3
    args:
      - /opt/jig-mcp/scripts/telegram_fetch.py
  darwin:
    command: python3
    args:
      - /opt/jig-mcp/scripts/telegram_fetch.py
  windows:
    command: powershell
    args:
      - -ExecutionPolicy
      - Bypass
      - -File
      - C:\jig-mcp\scripts\telegram_fetch.ps1

timeout: 30s
```

```python
# scripts/telegram_fetch.py
#!/usr/bin/env python3
"""Fetch messages from Telegram channels via Bot API."""

import sys
import json
import os
import urllib.request
import urllib.parse

METADATA = {
    "name": "telegram-fetch",
    "description": "Fetch messages from a Telegram channel or chat",
    "inputSchema": {
        "type": "object",
        "properties": {
            "chat_id": {
                "type": "string",
                "description": "Channel username (e.g., @channelname) or numeric chat ID"
            },
            "limit": {
                "type": "integer",
                "description": "Number of messages to fetch",
                "default": 10,
                "minimum": 1,
                "maximum": 100
            },
            "offset": {
                "type": "integer",
                "description": "Number of messages to skip",
                "default": 0
            }
        },
        "required": ["chat_id"]
    }
}

if "--mcp-metadata" in sys.argv:
    print(json.dumps(METADATA))
    sys.exit(0)

def main():
    bot_token = os.environ.get("TELEGRAM_BOT_TOKEN")
    if not bot_token:
        print(json.dumps({
            "content": [{"type": "text", "text": "TELEGRAM_BOT_TOKEN environment variable not set"}],
            "isError": True
        }))
        sys.exit(1)

    try:
        input_data = sys.stdin.read()
        args = json.loads(input_data)
    except json.JSONDecodeError as e:
        print(json.dumps({
            "content": [{"type": "text", "text": f"Invalid JSON input: {e}"}],
            "isError": True
        }))
        sys.exit(1)

    chat_id = args.get("chat_id", "")
    limit = min(max(args.get("limit", 10), 1), 100)
    offset = args.get("offset", 0)

    if not chat_id:
        print(json.dumps({
            "content": [{"type": "text", "text": "Missing required parameter: chat_id"}],
            "isError": True
        }))
        sys.exit(1)

    # Telegram Bot API doesn't support getting channel history without being a member
    # This is a simplified example - real implementation would need adjustments
    api_url = f"https://api.telegram.org/bot{bot_token}/getUpdates"

    try:
        # Note: Actual implementation depends on your use case
        # Channels require bot to be admin, groups require bot to be member
        result_text = f"Would fetch {limit} messages from {chat_id}"
        result_text += f"\n\nNote: Full implementation requires bot setup with Telegram"

        print(json.dumps({
            "content": [{"type": "text", "text": result_text}]
        }))

    except Exception as e:
        print(json.dumps({
            "content": [{"type": "text", "text": f"Telegram API error: {e}"}],
            "isError": True
        }))

if __name__ == "__main__":
    main()
```

### Example 3: Docker Inspect Tool

```yaml
# tools/docker-inspect/manifest.yaml
name: docker-inspect
description: Query Docker container status, logs, and configuration
inputSchema:
  type: object
  properties:
    container_id:
      type: string
      description: Container name or ID
    action:
      type: string
      description: Action to perform
      enum:
        - inspect
        - logs
        - stats
      default: inspect
    tail:
      type: integer
      description: Number of log lines (for logs action)
      default: 100
  required:
    - container_id

platforms:
  linux:
    command: bash
    args:
      - /opt/jig-mcp/scripts/docker_inspect.sh
  darwin:
    command: bash
    args:
      - /opt/jig-mcp/scripts/docker_inspect.sh

timeout: 30s
terminal:
  enabled: true
  allowlist:
    - docker inspect
    - docker logs
    - docker stats --no-stream
```

```bash
#!/bin/bash
# docker_inspect.sh - Query Docker containers

METADATA='{
    "name": "docker-inspect",
    "description": "Query Docker container status, logs, and configuration",
    "inputSchema": {
        "type": "object",
        "properties": {
            "container_id": {
                "type": "string",
                "description": "Container name or ID"
            },
            "action": {
                "type": "string",
                "description": "Action to perform",
                "enum": ["inspect", "logs", "stats"],
                "default": "inspect"
            },
            "tail": {
                "type": "integer",
                "description": "Number of log lines (for logs action)",
                "default": 100
            }
        },
        "required": ["container_id"]
    }
}'

if [[ "$1" == "--mcp-metadata" ]]; then
    echo "$METADATA"
    exit 0
fi

# Read JSON from stdin
INPUT=$(cat)

CONTAINER_ID=$(echo "$INPUT" | jq -r '.container_id // ""')
ACTION=$(echo "$INPUT" | jq -r '.action // "inspect"')
TAIL=$(echo "$INPUT" | jq -r '.tail // 100')

if [[ -z "$CONTAINER_ID" ]]; then
    jq -n '{
        content: [{type: "text", text: "Missing required parameter: container_id"}],
        isError: true
    }'
    exit 1
fi

case "$ACTION" in
    inspect)
        OUTPUT=$(docker inspect "$CONTAINER_ID" 2>&1)
        ;;
    logs)
        OUTPUT=$(docker logs --tail "$TAIL" "$CONTAINER_ID" 2>&1)
        ;;
    stats)
        OUTPUT=$(docker stats --no-stream "$CONTAINER_ID" 2>&1)
        ;;
    *)
        OUTPUT="Unknown action: $ACTION"
        ;;
esac

if [[ $? -ne 0 ]]; then
    jq -n --arg error "$OUTPUT" '{
        content: [{type: "text", text: $error}],
        isError: true
    }'
    exit 1
fi

jq -n --arg text "$OUTPUT" '{
    content: [{type: "text", text: $text}]
}'
```

---

## Quick Reference

### Minimum Viable Tool (Python)

```python
#!/usr/bin/env python3
import sys, json

METADATA = {
    "name": "hello-tool",
    "description": "A simple greeting tool",
    "inputSchema": {
        "type": "object",
        "properties": {
            "name": {"type": "string", "description": "Name to greet"}
        },
        "required": ["name"]
    }
}

if "--mcp-metadata" in sys.argv:
    print(json.dumps(METADATA))
    sys.exit(0)

args = json.loads(sys.stdin.read())
print(json.dumps({
    "content": [{"type": "text", "text": f"Hello, {args['name']}!"}]
}))
```

### Minimum Viable Manifest

```yaml
name: hello-tool
description: A simple greeting tool
inputSchema:
  type: object
  properties:
    name:
      type: string
      description: Name to greet
  required:
    - name
platforms:
  linux:
    command: python3
    args:
      - /path/to/hello.py
```

---

## Support

For issues or questions:
- jig-mcp repository: [github.com/snipspin/jig-mcp](https://github.com/snipspin/jig-mcp)
- MCP specification: [modelcontextprotocol.io](https://modelcontextprotocol.io)
