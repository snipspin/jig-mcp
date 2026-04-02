# docker_containers

Manage Docker containers: list, start, stop, restart, view logs, inspect, and remove. Supports both local and remote Docker hosts via SSH.

## Overview

The `docker_containers` tool provides full lifecycle management for Docker containers:

- **List containers** - View running and stopped containers
- **Start/Stop/Restart** - Control container state
- **View logs** - Fetch container logs with configurable tail
- **Inspect** - Get detailed container information
- **Remove** - Delete containers (with force option)
- **Remote support** - Manage containers on remote Docker hosts

### Use Cases

- Monitor running containers
- Restart failed services
- Debug container issues via logs
- Clean up stopped containers
- Manage remote Docker hosts via SSH

## Configuration

No environment variables required. The tool uses the local Docker socket by default.

### Optional Settings

```bash
# Remote Docker host (can also be passed per-request)
DOCKER_HOST=ssh://user@remote-server

# Docker socket path (for non-default locations)
DOCKER_SOCKET=/var/run/docker.sock
```

## Usage

### List Running Containers

```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "list"
  }
}
```

### List All Containers (Including Stopped)

```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "list",
    "all": true
  }
}
```

### Start a Container

```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "start",
    "container": "my-container"
  }
}
```

### Stop a Container

```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "stop",
    "container": "my-container"
  }
}
```

### View Container Logs

```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "logs",
    "container": "my-container",
    "tail": 50
  }
}
```

### Inspect a Container

```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "inspect",
    "container": "my-container"
  }
}
```

### Remove a Container

```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "remove",
    "container": "my-container"
  }
}
```

### Remote Docker Host

```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "list",
    "host": "ssh://user@192.168.1.100"
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `action` | string | Action: list, start, stop, restart, logs, inspect, remove | Yes |
| `container` | string | Container name or ID | Yes (except for list) |
| `host` | string | Remote Docker host (e.g., `ssh://user@server`) | No |
| `tail` | integer | Log lines to return (default: 100) | No (logs only) |
| `all` | boolean | Include stopped containers | No (list only) |

## Response Format

### List Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "CONTAINER ID   NAME       IMAGE         STATUS         PORTS\nabc123def456   my-app     nginx:latest  Up 2 hours     0.0.0.0:80->80/tcp\ndef789ghi012   redis      redis:alpine  Up 5 hours     6379/tcp"
    }
  ]
}
```

### Logs Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "2024-01-15 10:30:00 Server started on port 8080\n2024-01-15 10:30:01 Connected to database\n2024-01-15 10:30:02 Ready to accept connections"
    }
  ]
}
```

### Inspect Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "[\n  {\n    \"Id\": \"abc123def456\",\n    \"Name\": \"/my-app\",\n    \"State\": {\"Status\": \"running\", \"Running\": true},\n    \"Config\": {\"Image\": \"nginx:latest\"}\n  }\n]"
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
      "text": "Error: No such container: nonexistent-container"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: Check All Running Containers

**Request:**
```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "list",
    "all": false
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "CONTAINER ID   NAME              IMAGE         STATUS         PORTS\nabc123def456   web-server        nginx:latest  Up 2 hours     0.0.0.0:80->80/tcp\ndef789ghi012   cache             redis:alpine  Up 5 hours     6379/tcp\n123abc456def   app-backend       node:18       Up 1 hour      3000/tcp"
    }
  ]
}
```

### Example 2: Restart a Failed Container

**Request:**
```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "restart",
    "container": "web-server"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Container 'web-server' restarted successfully."
    }
  ]
}
```

### Example 3: Debug Application Issues

**Request:**
```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "logs",
    "container": "app-backend",
    "tail": 100
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Error: Connection refused to database\nRetrying in 5 seconds...\nError: Connection refused to database\nContainer restarting due to health check failure"
    }
  ]
}
```

### Example 4: Remote Container Management

**Request:**
```json
{
  "name": "docker_containers",
  "arguments": {
    "action": "list",
    "host": "ssh://admin@homelab.local"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "CONTAINER ID   NAME       IMAGE         STATUS\nxyz789abc123   pihole     pihole/pihole  Up 3 days\nabc123xyz789   nextcloud  nextcloud      Up 1 day"
    }
  ]
}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Cannot connect to Docker daemon" | Docker not running or socket inaccessible | Start Docker service, check socket permissions |
| "No such container" | Wrong container name/ID | Use `action: list` to find correct name |
| "Permission denied" | SSH authentication failed | Check SSH key, verify host format |
| "Timeout waiting for container" | Container stuck in stopping state | Use `remove` with force, then recreate |
| "Host not found" | Invalid remote host format | Use format `ssh://user@hostname` or `tcp://host:2375` |

## Security Notes

1. **Docker Socket Access**: Access to the Docker socket is equivalent to root access. Only trusted users should have access to jig-mcp when this tool is enabled.

2. **Remote Host Security**: When using remote Docker hosts:
   - Use SSH key authentication (not passwords)
   - Restrict SSH keys to specific commands if possible
   - Use TLS for TCP connections (`tcp://host:2375` is unencrypted)

3. **Container Removal**: The `remove` action uses `-f` (force) which kills running containers. Use with caution.

4. **Audit Logging**: All container operations are logged. Review logs for unauthorized access attempts.

5. **Resource Limits**: Container operations respect the tool's timeout (default: 30s). Long-running operations may be killed.

## Files

```
tools/docker_containers/
├── manifest.yaml           # Tool configuration
├── README.md               # This documentation
└── scripts/
    ├── docker_containers.sh    # Bash implementation (Linux/macOS)
    └── docker_containers.ps1   # PowerShell implementation (Windows)
```

## See Also

- [docker_images](../docker_images/README.md) - Manage Docker images
- [docker_compose](../docker_compose/README.md) - Multi-container orchestration
- [Docker CLI Reference](https://docs.docker.com/engine/reference/commandline/container/) - Official docs
- [Remote Docker](https://docs.docker.com/engine/reference/commandline/cli/#connection-helper-options) - Remote host setup
