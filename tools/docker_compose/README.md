# docker_compose

Manage Docker Compose stacks: start, stop, view status, logs, pull images, and restart services. Supports both local and remote Docker hosts via SSH.

## Overview

The `docker_compose` tool provides full Docker Compose stack management:

- **up** - Start a Compose stack in detached mode
- **down** - Stop and remove stack resources
- **ps** - List services in the stack
- **logs** - View service logs with configurable tail
- **pull** - Pull stack images without starting
- **restart** - Restart services in the stack
- **Remote support** - Manage stacks on remote Docker hosts

### Use Cases

- Deploy multi-container applications
- Manage development environments
- Start/stop entire application stacks
- Debug service issues via logs
- Pre-pull images before deployment
- Manage remote homelab services

## Configuration

No environment variables required. The tool uses the local Docker socket by default.

### Optional Settings

```bash
# Remote Docker host (can also be passed per-request)
DOCKER_HOST=ssh://user@remote-server

# Docker socket path (for non-default locations)
DOCKER_SOCKET=/var/run/docker.sock

# Default Compose project name (overrides directory-based naming)
COMPOSE_PROJECT_NAME=myapp
```

## Usage

### Start a Stack

```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "up",
    "project_dir": "/path/to/compose"
  }
}
```

### Start a Specific Service

```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "up",
    "project_dir": "/path/to/compose",
    "service": "web"
  }
}
```

### Stop and Remove a Stack

```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "down",
    "project_dir": "/path/to/compose"
  }
}
```

### List Stack Services

```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "ps",
    "project_dir": "/path/to/compose"
  }
}
```

### View Service Logs

```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "logs",
    "project_dir": "/path/to/compose",
    "service": "web",
    "tail": 50
  }
}
```

### Pull Stack Images

```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "pull",
    "project_dir": "/path/to/compose"
  }
}
```

### Restart a Service

```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "restart",
    "project_dir": "/path/to/compose",
    "service": "web"
  }
}
```

### Remote Docker Host

```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "ps",
    "project_dir": "/path/to/compose",
    "host": "ssh://user@192.168.1.100"
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `action` | string | Action: up, down, ps, logs, pull, restart | Yes |
| `project_dir` | string | Path to directory containing `docker-compose.yml` | Yes |
| `host` | string | Remote Docker host (e.g., `ssh://user@server`) | No |
| `service` | string | Target specific service (optional) | No |
| `tail` | integer | Log lines to return (default: 100) | No (logs only) |

## Response Format

### Up Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Stack in '/path/to/compose' started successfully.\n Container app-web-1  Started\n Container app-db-1   Started\n Container app-redis-1 Started"
    }
  ]
}
```

### Down Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Stack in '/path/to/compose' stopped and removed.\n Container app-web-1  Stopped\n Container app-db-1   Stopped\n Container app-web-1  Removed\n Container app-db-1   Removed\n Network app_default Removed"
    }
  ]
}
```

### Ps Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "NAME            IMAGE            COMMAND                  STATUS          PORTS\napp-web-1       nginx:latest     \"/docker-entrypoint\u2026\"   Up 2 hours      0.0.0.0:80->80/tcp\napp-db-1        postgres:15      \"docker-entrypoint\u2026\"    Up 2 hours      5432/tcp\napp-redis-1     redis:alpine     \"docker-entrypoint\u2026\"    Up 2 hours      6379/tcp"
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
      "text": "web-1  | 2024-01-15 10:30:00 Server started on port 80\ndb-1   | 2024-01-15 10:30:01 Database ready for connections\nredis-1| 2024-01-15 10:30:01 Ready to accept connections"
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
      "text": "Error starting stack: Cannot find docker-compose.yml in /path/to/compose"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: Deploy a Full Stack

**Request:**
```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "up",
    "project_dir": "/home/user/myapp"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Stack in '/home/user/myapp' started successfully.\n Container myapp-web-1    Started\n Container myapp-db-1     Started\n Container myapp-redis-1  Started\n Container myapp-worker-1 Started"
    }
  ]
}
```

### Example 2: Check Stack Status

**Request:**
```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "ps",
    "project_dir": "/home/user/myapp"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "NAME                IMAGE              STATUS              PORTS\nmyapp-web-1         nginx:latest       Up 2 hours           0.0.0.0:80->80/tcp\nmyapp-db-1          postgres:15        Up 2 hours           5432/tcp\nmyapp-redis-1       redis:alpine       Up 2 hours           6379/tcp\nmyapp-worker-1      myapp:latest       Up 2 hours"
    }
  ]
}
```

### Example 3: Debug Service Issues

**Request:**
```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "logs",
    "project_dir": "/home/user/myapp",
    "service": "worker",
    "tail": 50
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "worker-1 | 2024-01-15 10:30:00 Starting worker\nworker-1 | 2024-01-15 10:30:01 Connected to Redis\nworker-1 | 2024-01-15 10:30:02 Processing job #1234\nworker-1 | 2024-01-15 10:30:05 Job #1234 completed"
    }
  ]
}
```

### Example 4: Pre-pull Images Before Deployment

**Request:**
```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "pull",
    "project_dir": "/home/user/myapp"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Images pulled for stack in '/home/user/myapp'.\n myapp-web-1    Pulled\n myapp-db-1     Pulled\n myapp-redis-1  Pulled\n myapp-worker-1 Pulled"
    }
  ]
}
```

### Example 5: Restart a Single Service

**Request:**
```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "restart",
    "project_dir": "/home/user/myapp",
    "service": "web"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Stack in '/home/user/myapp' restarted.\n Container myapp-web-1  Restarted"
    }
  ]
}
```

### Example 6: Remote Stack Management

**Request:**
```json
{
  "name": "docker_compose",
  "arguments": {
    "action": "ps",
    "project_dir": "/opt/pihole",
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
      "text": "NAME            IMAGE                  STATUS        PORTS\npihole          pihole/pihole:latest   Up 3 days     0.0.0.0:53->53/tcp, 0.0.0.0:80->80/tcp"
    }
  ]
}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Cannot find docker-compose.yml" | Wrong project_dir or missing file | Verify path contains docker-compose.yml |
| "Cannot connect to Docker daemon" | Docker not running or socket inaccessible | Start Docker service, check socket permissions |
| "Service not found" | Invalid service name | Use `action: ps` to list valid service names |
| "Port already in use" | Another process using the port | Stop conflicting service or change port mapping |
| "Permission denied" | SSH authentication failed | Check SSH key, verify host format |
| "Container exited immediately" | Application error in container | Use `action: logs` to diagnose |

## Security Notes

1. **Docker Socket Access**: Access to the Docker socket provides root-equivalent privileges. Only trusted users should access jig-mcp when this tool is enabled.

2. **Project Directory Validation**: The tool requires an explicit `project_dir` containing a `docker-compose.yml` file. This prevents arbitrary stack deployments.

3. **Remote Host Security**: When using remote Docker hosts:
   - Use SSH key authentication (not passwords)
   - Restrict SSH keys to specific commands if possible
   - Use TLS for TCP connections

4. **Volume Mounts**: Compose files may mount sensitive host paths. Review compose files before deployment.

5. **Network Exposure**: Compose stacks may expose ports to the host. Review port mappings in the compose file.

6. **Audit Logging**: All Compose operations are logged. Review logs for unauthorized stack deployments.

## Files

```
tools/docker_compose/
├── manifest.yaml           # Tool configuration
├── README.md               # This documentation
└── scripts/
    ├── docker_compose.sh   # Bash implementation (Linux/macOS)
    └── docker_compose.ps1  # PowerShell implementation (Windows)
```

## See Also

- [docker_containers](../docker_containers/README.md) - Manage individual containers
- [docker_images](../docker_images/README.md) - Manage Docker images
- [Docker Compose Reference](https://docs.docker.com/compose/reference/) - Official docs
- [Compose File Format](https://docs.docker.com/compose/compose-file/) - YAML specification
