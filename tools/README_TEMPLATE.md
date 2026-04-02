# {{tool_name}}

{{One-line description of what the tool does}}

## Overview

{{2-3 paragraphs explaining the tool's purpose, use cases, and key features}}

## Configuration

{{List any required or optional environment variables}}

```bash
# Required settings
EXAMPLE_VAR=value

# Optional settings
OPTIONAL_VAR=default_value
```

## Usage

### Basic Example

```json
{
  "name": "{{tool_name}}",
  "arguments": {
    "{{primary_param}}": "value"
  }
}
```

### Advanced Options

```json
{
  "name": "{{tool_name}}",
  "arguments": {
    "{{primary_param}}": "value",
    "optional_param": "value"
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `param_name` | string | Description | Yes |
| `optional_param` | integer | Description | No |

## Response Format

{{Describe the response structure with an example}}

```json
{
  "result": "example response"
}
```

## Examples

### Example 1: Basic Usage

**Request:**
```json
{"name": "{{tool_name}}", "arguments": {"param": "value"}}
```

**Response:**
```json
{"result": "response"}
```

### Example 2: Advanced Usage

{{Another practical example}}

## Troubleshooting

### Common Issues

| Error | Cause | Solution |
|-------|-------|----------|
| Error message | What causes it | How to fix |

## Security Notes

{{Any security considerations specific to this tool}}

## Files

```
tools/{{tool_name}}/
├── manifest.yaml    # Tool configuration
├── README.md        # This documentation
└── scripts/         # (Optional) Wrapper scripts
```

## See Also

- [Tool Developer Guide](../../docs/TOOL_DEVELOPER_GUIDE.md)
- [Configuration Reference](../../README.md#configuration-reference)
- {{Related links}}
