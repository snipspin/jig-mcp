# remote_docker_compose

Manage Docker Compose stacks on remote hosts via SSH: start, stop, view status, logs, pull images, and restart services. Combines SSH key-based authentication with Docker Compose CLI for remote stack management.

## Overview

`remote_docker_compose` provides full Docker Compose stack management on remote hosts:

- **All Compose operations** - up, down, ps, logs, pull, restart
- **Service-specific targeting** - Operate on individual services within a stack
- **SSH key authentication** - Secure, passwordless remote access
- **Log tailing** - Configurable log line count per service
- **Platform support** - Bash on Linux/macOS, PowerShell on Windows

### Use Cases

- Deploy multi-container applications on remote servers
- Manage homelab stacks (Pi-hole, Nextcloud, etc.)
- Restart specific services without full stack restart
- Monitor stack health via remote status checks
- Pre-pull images before deployment

### How It Works

The tool SSHs into the remote host and executes Docker Compose CLI commands:
```bash
ssh user@host "docker compose -f /path/to/docker-compose.yml up -d"
ssh user@host "docker compose -f /path/to/docker-compose.yml logs --tail 100"
```

## Configuration

No environment variables required. Uses system SSH configuration.

### Optional Settings

```bash
# Default SSH key (can also be passed per-request)
SSH_KEY_PATH=/home/user/.ssh/homelab_key

# SSH agent socket
SSH_AUTH_SOCK=/run/user/1000/ssh-agent.socket
```

## Usage

### Start a Stack

```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "up",
    "host": "admin@homelab.local",
    "project_dir": "/opt/pihole"
  }
}
```

### Start a Specific Service

```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "up",
    "host": "admin@homelab.local",
    "project_dir": "/opt/myapp",
    "service": "web"
  }
}
```

### Stop and Remove a Stack

```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "down",
    "host": "admin@homelab.local",
    "project_dir": "/opt/pihole"
  }
}
```

### List Stack Services

```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "ps",
    "host": "admin@homelab.local",
    "project_dir": "/opt/myapp"
  }
}
```

### View Service Logs

```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "logs",
    "host": "admin@homelab.local",
    "project_dir": "/opt/myapp",
    "service": "db",
    "tail": 50
  }
}
```

### Pull Stack Images

```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "pull",
    "host": "admin@homelab.local",
    "project_dir": "/opt/myapp"
  }
}
```

### Restart a Service

```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "restart",
    "host": "admin@homelab.local",
    "project_dir": "/opt/myapp",
    "service": "web"
  }
}
```

### With Custom SSH Key

```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "ps",
    "host": "user@server.local",
    "project_dir": "/opt/app",
    "key": "/home/user/.ssh/server_key"
  }
}
```

### With Custom SSH Port

```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "up",
    "host": "user@server.local",
    "project_dir": "/opt/app",
    "port": 2222
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `action` | string | Action: up, down, ps, logs, pull, restart | Yes |
| `host` | string | Remote host in format `user@hostname` or `hostname` | Yes |
| `project_dir` | string | Path to directory containing `docker-compose.yml` | Yes |
| `service` | string | Target specific service (optional) | No |
| `key` | string | Path to SSH private key file | No |
| `port` | integer | SSH port (default: 22) | No |
| `tail` | integer | Log lines to return (default: 100) | No (logs only) |

## Response Format

### Up Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Stack in '/opt/pihole' started successfully.\n Container pihole-1  Started"
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
      "text": "NAME           IMAGE            COMMAND                  STATUS          PORTS\npihole-1       pihole/pihole    "/init"                  Up 3 days       53/tcp, 0.0.0.0:80->80/tcp"
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
      "text": "pihole-1  | 2024-01-15 10:30:00 Starting pihole-FTL\npihole-1  | 2024-01-15 10:30:01 DNS server started on port 53"
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
      "text": "Error starting stack: Cannot find docker-compose.yml in /opt/invalid"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: Deploy Pi-hole Stack

**Request:**
```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "up",
    "host": "admin@homelab.local",
    "project_dir": "/opt/pihole"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Stack in '/opt/pihole' started successfully.\n Container pihole-1  Started"
    }
  ]
}
```

### Example 2: Check Stack Status

**Request:**
```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "ps",
    "host": "admin@homelab.local",
    "project_dir": "/opt/nextcloud"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "NAME              IMAGE         STATUS          PORTS\nnextcloud-app-1   nextcloud     Up 2 days       0.0.0.0:8080->80/tcp\nnextcloud-db-1    mariadb       Up 2 days       3306/tcp"
    }
  ]
}
```

### Example 3: Debug Service Issues

**Request:**
```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "logs",
    "host": "admin@homelab.local",
    "project_dir": "/opt/nextcloud",
    "service": "app",
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
      "text": "nextcloud-app-1  | AH00558: apache2: Could not reliably determine the server's fully qualified domain name\nnextcloud-app-1  | [Fri Jan 12 10:30:00.123456 2024] [mpm_prefork:notice] AH00163: Apache/2.4.52 configured\nnextcloud-app-1  | [Fri Jan 12 10:30:01.234567 2024] [core:error] Database connection failed"
    }
  ]
}
```

### Example 4: Pull Updated Images

**Request:**
```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "pull",
    "host": "admin@homelab.local",
    "project_dir": "/opt/myapp"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Images pulled for stack in '/opt/myapp'.\n myapp-web-1    Pulled\n myapp-db-1     Pulled"
    }
  ]
}
```

### Example 5: Restart Single Service

**Request:**
```json
{
  "name": "remote_docker_compose",
  "arguments": {
    "action": "restart",
    "host": "admin@homelab.local",
    "project_dir": "/opt/myapp",
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
      "text": "Stack in '/opt/myapp' restarted.\n Container myapp-web-1  Restarted"
    }
  ]
}
```

### Example 6: Full Stack Redeploy

**Request (sequential operations):**
```json
// Step 1: Pull new images
{"name": "remote_docker_compose", "arguments": {"action": "pull", "host": "admin@homelab.local", "project_dir": "/opt/myapp"}}

// Step 2: Restart stack with new images
{"name": "remote_docker_compose", "arguments": {"action": "restart", "host": "admin@homelab.local", "project_dir": "/opt/myapp"}}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Cannot find docker-compose.yml" | Wrong project_dir or missing file | Verify path contains docker-compose.yml |
| "Cannot connect to Docker daemon" | Docker not running on remote host | Start Docker: `sudo systemctl start docker` |
| "Permission denied" | SSH authentication failed | Check SSH key, verify authorized_keys |
| "Command not found: docker compose" | Docker Compose not installed | Install: `sudo apt install docker-compose-plugin` |
| "Service not found" | Invalid service name | Use `action: ps` to list valid services |
| "Port already in use" | Another process using the port | Stop conflicting service or change port |
| "Host key verification failed" | Host key changed | Remove old key from `~/.ssh/known_hosts` |

## Security Notes

1. **SSH Key Security**: Use dedicated SSH keys for jig-mcp. Store keys securely and never commit to version control.

2. **Docker Group Membership**: Remote users must be in the `docker` group for passwordless Docker access. This provides root-equivalent privileges.

3. **Project Directory Validation**: The tool requires an explicit `project_dir` containing a `docker-compose.yml` file. This prevents arbitrary stack deployments.

4. **Host Key Verification**: Uses `StrictHostKeyChecking=accept-new`:
   - Accepts new keys on first connection
   - Rejects changed keys (MITM protection)

5. **Compose File Review**: Review docker-compose.yml files before deployment. They may:
   - Mount sensitive host paths as volumes
   - Expose ports to the network
   - Set environment variables with secrets

6. **Audit Logging**: All Compose operations are logged with host, project_dir, action, and service name.

## SSH Key Setup for Remote Compose

### 1. Generate SSH Key

```bash
ssh-keygen -t ed25519 -C "jig-mcp-compose" -f ~/.ssh/jig-mcp-compose
```

### 2. Copy to Remote Host

```bash
ssh-copy-id -i ~/.ssh/jig-mcp-compose.pub user@remote-host
```

### 3. Add User to Docker Group (on remote host)

```bash
ssh user@remote-host "sudo usermod -aG docker $USER"
```

### 4. Verify Docker Compose Installation

```bash
ssh user@remote-host "docker compose version"
```

## Files

```
tools/remote_docker_compose/
├── manifest.yaml                       # Tool configuration
├── README.md                           # This documentation
└── scripts/
    ├── remote_docker_compose.sh        # Bash implementation (Linux/macOS)
    └── remote_docker_compose.ps1       # PowerShell implementation (Windows)
```

## See Also

- [docker_compose](../docker_compose/README.md) - Local Docker Compose management
- [ssh_exec](../ssh_exec/README.md) - Generic SSH command execution
- [remote_docker_containers](../remote_docker_containers/README.md) - Remote container management
- [Docker Compose Reference](https://docs.docker.com/compose/reference/) - Official docs
