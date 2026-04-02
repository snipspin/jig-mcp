# api_bridge

Generic HTTP/REST client for making requests to any API endpoint. Supports all common HTTP methods, custom headers, request bodies, and authentication via environment variables.

## Overview

`api_bridge` is a flexible HTTP client that allows jig-mcp to interact with external APIs and web services. It provides:

- **Full HTTP support** - GET, POST, PUT, DELETE methods
- **Custom headers** - Add any HTTP headers needed
- **Request bodies** - Send JSON or string payloads
- **SSRF protection** - URL allowlisting prevents unauthorized access
- **Authentication** - Basic Auth and Bearer token support via manifest env vars

### Use Cases

- Query external APIs (GitHub, weather services, payment gateways)
- Webhook notifications and callbacks
- REST API testing and debugging
- Integration with internal homelab services
- Fetching data from microservices

## Configuration

`api_bridge` uses manifest-level environment variables for sensitive configuration. Add these to your `.env` file:

### Required Settings

No required environment variables - the tool works with just the URL parameter.

### Optional Settings

```bash
# Bearer token for authenticated requests
API_BRIDGE_BEARER_TOKEN=your_bearer_token_here

# Basic Auth credentials
API_BRIDGE_BASIC_AUTH_USER=username
API_BRIDGE_BASIC_AUTH_PASS=password

# SSRF protection - allowed URL prefixes (recommended)
# Only URLs starting with these prefixes will be allowed
API_BRIDGE_ALLOWED_PREFIXES=https://api.github.com,https://api.openweathermap.org
```

### SSRF Protection

The `allowedURLPrefixes` in `manifest.yaml` restricts which URLs the bridge can reach. This prevents the LLM from probing internal services.

**Example configurations:**

```yaml
# GitHub API only
http:
  allowedURLPrefixes:
    - "https://api.github.com"

# Weather API only
http:
  allowedURLPrefixes:
    - "https://api.openweathermap.org"

# Local homelab service (if truly needed)
http:
  allowedURLPrefixes:
    - "http://192.168.1.100:8080/api"

# Multiple APIs
http:
  allowedURLPrefixes:
    - "https://api.github.com"
    - "https://api.stripe.com"
```

## Usage

### Basic GET Request

```json
{
  "name": "api_bridge",
  "arguments": {
    "url": "https://api.github.com/repos/anthropics/claude-code"
  }
}
```

### POST Request with JSON Body

```json
{
  "name": "api_bridge",
  "arguments": {
    "url": "https://api.example.com/webhook",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json"
    },
    "body": "{\"event\": \"deploy\", \"status\": \"success\"}"
  }
}
```

### Authenticated Request

```json
{
  "name": "api_bridge",
  "arguments": {
    "url": "https://api.github.com/user/repos",
    "method": "GET",
    "headers": {
      "Accept": "application/vnd.github.v3+json"
    }
  }
}
```

Authentication headers are added automatically from manifest environment variables.

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `url` | string | The full destination URL | Yes |
| `method` | string | HTTP method: GET, POST, PUT, DELETE | No (default: GET) |
| `headers` | object | Custom HTTP headers | No |
| `body` | string | Request body (string or JSON) | No |

## Response Format

The response contains the raw HTTP response body as text:

```json
{
  "content": [
    {
      "type": "text",
      "text": "{\"id\": 12345, \"name\": \"repo-name\", \"full_name\": \"owner/repo\"...}"
    }
  ]
}
```

For error responses, the HTTP status and body are returned with `isError: true`:

```json
{
  "content": [
    {
      "type": "text",
      "text": "HTTP 404 Not Found: {\"message\": \"Not Found\"}"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: GitHub Repository Info

**Request:**
```json
{
  "name": "api_bridge",
  "arguments": {
    "url": "https://api.github.com/repos/golang/go",
    "headers": {
      "Accept": "application/vnd.github.v3+json"
    }
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "{\"id\": 6820896, \"name\": \"go\", \"full_name\": \"golang/go\", \"private\": false, ...}"
    }
  ]
}
```

### Example 2: Webhook Notification

**Request:**
```json
{
  "name": "api_bridge",
  "arguments": {
    "url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json"
    },
    "body": "{\"text\": \"Deployment completed successfully!\"}"
  }
}
```

### Example 3: Weather API Query

**Request:**
```json
{
  "name": "api_bridge",
  "arguments": {
    "url": "https://api.openweathermap.org/data/2.5/weather?q=London&appid=YOUR_API_KEY",
    "method": "GET"
  }
}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "URL does not match any allowed prefix" | SSRF protection blocking request | Add URL prefix to `allowedURLPrefixes` in manifest |
| "401 Unauthorized" | Missing or invalid authentication | Check bearer token or basic auth credentials in manifest |
| "403 Forbidden" | API key invalid or rate limited | Verify API key, check rate limit headers |
| "Connection refused" | Endpoint unreachable | Verify URL, check network/firewall |
| "404 Not Found" | Resource doesn't exist | Check URL path and parameters |
| "Request timeout" | Slow or unresponsive endpoint | Increase timeout in manifest (default: 15s) |

## Security Notes

1. **SSRF Protection**: Always configure `allowedURLPrefixes` to restrict which URLs the bridge can access. Without this, the tool could potentially access internal services.

2. **API Key Security**: Store API keys and tokens in your `.env` file (never commit to git). Use manifest-level environment variables:
   ```yaml
   env:
     API_BRIDGE_BEARER_TOKEN: "${API_BRIDGE_BEARER_TOKEN}"
   ```

3. **Least Privilege**: Only allow the specific API endpoints your use case requires. Avoid wildcards like `https://api.example.com/` if you can be more specific.

4. **Response Handling**: API responses may contain sensitive data. The full response body is returned - ensure downstream consumers handle this appropriately.

5. **Rate Limiting**: Be aware of API rate limits. Consider implementing delays between requests in automated workflows.

## Files

```
tools/api_bridge/
├── manifest.yaml    # Tool configuration, SSRF rules, timeouts
├── README.md        # This documentation
└── scripts/         # (Optional) Custom wrapper scripts
```

## See Also

- [Tool Developer Guide](../../docs/TOOL_DEVELOPER_GUIDE.md) - Building custom tools
- [Configuration Reference](../../README.md#configuration-reference) - Environment variables
- [SSRF Prevention](../../docs/SECURITY.md#ssrf-prevention) - Security best practices
- [HTTP Tool Implementation](../../internal/tools/http.go) - Source code
