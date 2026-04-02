# remote_docker_containers

Manage Docker containers on remote hosts via SSH: list, start, stop, restart, view logs, inspect, and remove. Combines SSH key-based authentication with Docker CLI for remote container management.

## Overview

`remote_docker_containers` provides full Docker container lifecycle management on remote hosts:

- **All container operations** - list, start, stop, restart, logs, inspect, remove
- **SSH key authentication** - Secure, passwordless remote access
- **Custom SSH ports** - Support for non-standard SSH configurations
- **Log tailing** - Configurable log line count
- **Platform support** - Bash on Linux/macOS, PowerShell on Windows

### Use Cases

- Manage containers on homelab servers
- Deploy and restart services on remote hosts
- Debug container issues via remote logs
- Monitor container status across multiple hosts
- Clean up stopped containers remotely

### How It Works

The tool SSHs into the remote host and executes Docker CLI commands:
```bash
ssh user@host "docker ps"
ssh user@host "docker logs --tail 100 container-name"
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

### List Running Containers

```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "list",
    "host": "user@192.168.1.100"
  }
}
```

### List All Containers (Including Stopped)

```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "list",
    "host": "admin@homelab.local",
    "all": true
  }
}
```

### Start a Container

```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "start",
    "host": "admin@homelab.local",
    "container": "nginx-proxy"
  }
}
```

### Stop a Container

```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "stop",
    "host": "admin@homelab.local",
    "container": "old-service"
  }
}
```

### View Container Logs

```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "logs",
    "host": "admin@homelab.local",
    "container": "web-app",
    "tail": 50
  }
}
```

### Restart a Container

```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "restart",
    "host": "admin@homelab.local",
    "container": "failing-service"
  }
}
```

### Remove a Container

```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "remove",
    "host": "admin@homelab.local",
    "container": "old-container"
  }
}
```

### With Custom SSH Key

```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "list",
    "host": "user@server.local",
    "key": "/home/user/.ssh/server_key"
  }
}
```

### With Custom SSH Port

```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "list",
    "host": "user@server.local",
    "port": 2222
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `action` | string | Action: list, start, stop, restart, logs, inspect, remove | Yes |
| `host` | string | Remote host in format `user@hostname` or `hostname` | Yes |
| `container` | string | Container name or ID | Yes (except list) |
| `key` | string | Path to SSH private key file | No |
| `port` | integer | SSH port (default: 22) | No |
| `tail` | integer | Log lines to return (default: 100) | No (logs only) |
| `all` | boolean | Include stopped containers | No (list only) |

## Response Format

### List Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "CONTAINER ID   NAME         IMAGE          STATUS         PORTS\nabc123def456   pihole       pihole/pihole  Up 3 days      0.0.0.0:53->53/tcp, 0.0.0.0:80->80/tcp"
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
      "text": "2024-01-15 10:30:00 Starting Pi-hole FTL\n2024-01-15 10:30:01 Database initialized\n2024-01-15 10:30:02 Ready to process DNS queries"
    }
  ]
}
```

### Success Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Container 'nginx-proxy' restarted successfully."
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
      "text": "Error starting container: No such container: nonexistent"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: Check Containers on Homelab Server

**Request:**
```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "list",
    "host": "admin@homelab.local"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "CONTAINER ID   NAME          IMAGE            STATUS        PORTS\nabc123def456   pihole        pihole/pihole    Up 3 days     53/tcp, 0.0.0.0:80->80/tcp\ndef789ghi012   nextcloud     nextcloud        Up 1 day      0.0.0.0:8080->80/tcp\n123abc456def   homeassistant home-assistant   Up 5 days     0.0.0.0:8123->8123/tcp"
    }
  ]
}
```

### Example 2: Restart Failing Service

**Request:**
```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "restart",
    "host": "admin@homelab.local",
    "container": "homeassistant"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Container 'homeassistant' restarted successfully."
    }
  ]
}
```

### Example 3: Debug Application Issues

**Request:**
```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "logs",
    "host": "admin@homelab.local",
    "container": "nextcloud",
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
      "text": "AH00558: apache2: Could not reliably determine the server's fully qualified domain name\nAH00558: apache2: Could not reliably determine the server's fully qualified domain name\n[Thu Jan 15 10:30:00.123456 2024] [mpm_prefork:notice] [pid 1] AH00163: Apache/2.4.52 configured -- resuming normal operations"
    }
  ]
}
```

### Example 4: Clean Up Stopped Containers

**Request:**
```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "list",
    "host": "admin@homelab.local",
    "all": true
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "CONTAINER ID   NAME         IMAGE          STATUS\nabc123def456   pihole       pihole/pihole  Up 3 days\ndeadbeef1234   old-backup   backup:latest  Exited (0) 2 days ago"
    }
  ]
}
```

**Follow-up (remove stopped container):**
```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "remove",
    "host": "admin@homelab.local",
    "container": "old-backup"
  }
}
```

### Example 5: Inspect Container Details

**Request:**
```json
{
  "name": "remote_docker_containers",
  "arguments": {
    "action": "inspect",
    "host": "admin@homelab.local",
    "container": "pihole"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "[\n  {\n    \"Id\": \"abc123def456\",\n    \"Name\": \"/pihole\",\n    \"State\": {\"Status\": \"running\", \"Running\": true},\n    \"NetworkSettings\": {\"Ports\": {\"53/tcp\": null, \"80/tcp\": [{\"HostPort\": \"80\"}]}}\n  }\n]"
    }
  ]
}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Cannot connect to Docker daemon" | Docker not running on remote host | Start Docker: `sudo systemctl start docker` |
| "Permission denied" | SSH authentication failed | Check SSH key, verify it's in `~/.ssh/authorized_keys` |
| "Got permission denied while trying to connect to the Docker daemon socket" | User not in docker group | Add user to docker group: `sudo usermod -aG docker $USER` |
| "No such container" | Wrong container name/ID | Use `action: list` to find correct name |
| "Connection refused" | SSH not running or wrong port | Verify SSH daemon, check port number |
| "Host key verification failed" | Host key changed | Remove old key from `~/.ssh/known_hosts` |

## Security Notes

1. **SSH Key Security**: Use dedicated SSH keys for jig-mcp. Never commit keys to version control.

2. **Docker Group Membership**: Remote users must be in the `docker` group for passwordless Docker access. This is equivalent to root access - only trust users with SSH access.

3. **Host Key Verification**: Uses `StrictHostKeyChecking=accept-new`:
   - Accepts new keys on first connection
   - Rejects changed keys (MITM protection)

4. **Command Execution**: The tool executes Docker CLI commands on the remote host. Container names are quoted to prevent injection.

5. **Audit Logging**: All remote Docker operations are logged with host, action, and container name.

6. **Network Security**: SSH traffic should be on a trusted network. Consider using a VPN for remote management over the internet.

## SSH Key Setup for Remote Docker

### 1. Generate SSH Key

```bash
ssh-keygen -t ed25519 -C "jig-mcp-docker" -f ~/.ssh/jig-mcp-docker
```

### 2. Copy to Remote Host

```bash
ssh-copy-id -i ~/.ssh/jig-mcp-docker.pub user@remote-host
```

### 3. Add User to Docker Group (on remote host)

```bash
ssh user@remote-host "sudo usermod -aG docker $USER"
```

### 4. Test Connection

```bash
ssh -i ~/.ssh/jig-mcp-docker user@remote-host "docker ps"
```

## Files

```
tools/remote_docker_containers/
├── manifest.yaml                   # Tool configuration
├── README.md                       # This documentation
└── scripts/
    ├── remote_docker_containers.sh # Bash implementation (Linux/macOS)
    └── remote_docker_containers.ps1 # PowerShell implementation (Windows)
```

## See Also

- [docker_containers](../docker_containers/README.md) - Local Docker management
- [ssh_exec](../ssh_exec/README.md) - Generic SSH command execution
- [remote_docker_compose](../remote_docker_compose/README.md) - Remote Compose via SSH
- [Docker Remote API](https://docs.docker.com/engine/api/) - Docker API documentation
