# service_health

Check service availability via ping, TCP port check, or HTTP health endpoint. A versatile health monitoring tool for homelab services and network diagnostics.

## Overview

`service_health` provides three types of health checks:

- **Ping** - ICMP echo to verify host reachability
- **Port** - TCP connection test to verify service is listening
- **HTTP** - HTTP/HTTPS request to check web service health

### Use Cases

- Monitor homelab service availability (Pi-hole, Nextcloud, etc.)
- Verify services are running after deployment
- Debug network connectivity issues
- Pre-flight checks before API calls
- Automated uptime monitoring

### Health Check Types

| Type | What It Tests | Use Case |
|------|---------------|----------|
| `ping` | ICMP echo response | Host is online and reachable |
| `port` | TCP connection | Service is listening on port |
| `http` | HTTP response | Web service is responding correctly |

## Configuration

No environment variables required.

### Optional Settings

```bash
# Default timeout for all checks (can also be passed per-request)
DEFAULT_HEALTH_CHECK_TIMEOUT=10
```

## Usage

### Ping Check

```json
{
  "name": "service_health",
  "arguments": {
    "check": "ping",
    "target": "192.168.1.100"
  }
}
```

### Port Check

```json
{
  "name": "service_health",
  "arguments": {
    "check": "port",
    "target": "192.168.1.100",
    "port": 22
  }
}
```

### HTTP Check (Default Path)

```json
{
  "name": "service_health",
  "arguments": {
    "check": "http",
    "target": "example.com"
  }
}
```

### HTTPS Check with Custom Path

```json
{
  "name": "service_health",
  "arguments": {
    "check": "http",
    "target": "api.example.com",
    "scheme": "https",
    "port": 443,
    "path": "/health"
  }
}
```

### With Custom Timeout

```json
{
  "name": "service_health",
  "arguments": {
    "check": "ping",
    "target": "192.168.1.100",
    "timeout": 10
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `check` | string | Check type: `ping`, `port`, `http` | Yes |
| `target` | string | Hostname or IP address | Yes |
| `port` | integer | TCP port number | Yes for `port`, optional for `http` |
| `path` | string | URL path for HTTP check (default: `/`) | No |
| `scheme` | string | HTTP scheme: `http` or `https` (default: `http`) | No |
| `timeout` | integer | Timeout in seconds (default: 5) | No |

## Response Format

### Healthy Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "HEALTHY: 192.168.1.100 is reachable.\n\nPING 192.168.1.100 (192.168.1.100) 56(84) bytes of data.\n64 bytes from 192.168.1.100: icmp_seq=1 ttl=64 time=0.523 ms\n64 bytes from 192.168.1.100: icmp_seq=2 ttl=64 time=0.891 ms\n64 bytes from 192.168.1.100: icmp_seq=3 ttl=64 time=0.765 ms\n\n--- 192.168.1.100 ping statistics ---\n3 packets transmitted, 3 received, 0% packet loss, time 2002ms"
    }
  ]
}
```

### Unhealthy Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "UNHEALTHY: 192.168.1.999 is not reachable.\n\nping: 192.168.1.999: Name or service not known"
    }
  ],
  "isError": true
}
```

### HTTP Healthy Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "HEALTHY: http://192.168.1.100:80/health returned HTTP 200.\n\nResponse (truncated):\n{\"status\": \"ok\", \"uptime\": 3600}"
    }
  ]
}
```

### HTTP Unhealthy Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "UNHEALTHY: http://192.168.1.100:8080/ returned HTTP 503.\n\nResponse (truncated):\nservice unavailable"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: Check Pi-hole Availability

**Request:**
```json
{
  "name": "service_health",
  "arguments": {
    "check": "ping",
    "target": "pihole.local"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "HEALTHY: pihole.local is reachable.\n\nPING pihole.local (192.168.1.10) 56(84) bytes of data.\n64 bytes from pihole.local: icmp_seq=1 ttl=64 time=1.23 ms\n\n--- pihole.local ping statistics ---\n3 packets transmitted, 3 received, 0% packet loss, time 2002ms"
    }
  ]
}
```

### Example 2: Verify SSH Service

**Request:**
```json
{
  "name": "service_health",
  "arguments": {
    "check": "port",
    "target": "server.local",
    "port": 22
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "HEALTHY: server.local:22 is open and accepting connections."
    }
  ]
}
```

### Example 3: Check Web Service Health

**Request:**
```json
{
  "name": "service_health",
  "arguments": {
    "check": "http",
    "target": "nextcloud.local",
    "scheme": "http",
    "path": "/status.php"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "HEALTHY: http://nextcloud.local/status.php returned HTTP 200.\n\nResponse (truncated):\n{\"installed\":true,\"version\":\"28.0.1.1\"}"
    }
  ]
}
```

### Example 4: Check HTTPS Endpoint

**Request:**
```json
{
  "name": "service_health",
  "arguments": {
    "check": "http",
    "target": "api.example.com",
    "scheme": "https",
    "port": 443,
    "path": "/api/health"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "HEALTHY: https://api.example.com:443/api/health returned HTTP 200.\n\nResponse (truncated):\n{\"healthy\":true,\"checks\":{\"database\":\"ok\",\"cache\":\"ok\"}}"
    }
  ]
}
```

### Example 5: Monitor Multiple Services

**Request (parallel execution):**
```json
[
  {"name": "service_health", "arguments": {"check": "ping", "target": "pihole.local"}},
  {"name": "service_health", "arguments": {"check": "port", "target": "nas.local", "port": 445}},
  {"name": "service_health", "arguments": {"check": "http", "target": "grafana.local", "scheme": "http"}}
]
```

**Response:**
```json
{
  "content": [
    {"type": "text", "text": "HEALTHY: pihole.local is reachable.\n..."},
    {"type": "text", "text": "HEALTHY: nas.local:445 is open and accepting connections."},
    {"type": "text", "text": "HEALTHY: http://grafana.local:3000/ returned HTTP 200.\n..."}
  ]
}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "UNHEALTHY: not reachable" | Host down or firewall blocking | Verify host is up, check firewall rules |
| "Connection refused" | No service listening on port | Start the service or check port number |
| "Connection timed out" | Firewall or network issue | Check network path, firewall rules |
| "HTTP 000" | Connection failed before response | Verify service is running, check URL |
| "Neither curl nor wget found" | Missing HTTP client | Install curl or wget |
| "Name or service not known" | DNS resolution failed | Check DNS settings, use IP address |

## Security Notes

1. **Read-Only Operation**: This tool only performs network diagnostics. It cannot modify system state.

2. **ICMP Considerations**: Some hosts block ICMP (ping) requests. A failed ping doesn't always mean the host is down.

3. **HTTP Response Truncation**: HTTP responses are truncated to 2KB to prevent memory issues. Full responses are not logged.

4. **Credential Handling**: Do not include credentials in URLs. Use authentication headers or tokens for protected endpoints.

5. **SSRF Protection**: Be cautious when allowing untrusted users to specify targets. Consider implementing target allowlisting for production use.

6. **Audit Logging**: All health checks are logged with target, check type, and result.

## Implementation Details

### Ping Check

- Uses `ping -c 3 -W timeout` on Linux/macOS
- Uses `Test-Connection` on PowerShell/Windows
- Sends 3 ICMP echo requests
- Returns full ping output including statistics

### Port Check

- Uses `nc -z -w timeout` (netcat) if available
- Falls back to `/dev/tcp` bash built-in
- Uses `Test-NetConnection` on PowerShell/Windows
- Tests TCP connection only (no data sent)

### HTTP Check

- Prefers `curl` if available
- Falls back to `wget` if curl unavailable
- Uses `Invoke-WebRequest` on PowerShell/Windows
- Returns HTTP status code and body (truncated)
- HTTP 2xx and 3xx are considered healthy
- HTTP 4xx and 5xx are considered unhealthy

## Files

```
tools/service_health/
├── manifest.yaml           # Tool configuration
├── README.md               # This documentation
└── scripts/
    ├── service_health.sh   # Bash implementation (Linux/macOS)
    └── service_health.ps1  # PowerShell implementation (Windows)
```

## See Also

- [system_info](../system_info/README.md) - System information tool
- [api_bridge](../api_bridge/README.md) - HTTP API client
- [ping(8)](https://man7.org/linux/man-pages/man8/ping.8.html) - Linux ping command
- [nc(1)](https://man7.org/linux/man-pages/man1/nc.1.html) - netcat command
