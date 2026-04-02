# ssh_exec

Execute commands on remote hosts via SSH with key-based authentication. The "escape hatch" for any remote operation on your homelab or infrastructure.

## Overview

`ssh_exec` provides generic SSH command execution to any remote host:

- **Key-based authentication** - Supports SSH keys, ssh-agent, or default keys
- **Custom ports** - Connect to non-standard SSH ports
- **No password prompts** - Batch mode ensures non-interactive operation
- **Automatic host key management** - Accepts new host keys on first connection
- **Platform support** - Bash on Linux/macOS, PowerShell on Windows

### Use Cases

- Run ad-hoc commands on remote servers
- Execute deployment scripts
- Fetch system information from multiple hosts
- Automate homelab management tasks
- Remote monitoring and diagnostics

### How It Works

The tool wraps the `ssh` command with sensible defaults:
- Strict host key checking enabled (accepts new keys automatically)
- 10-second connection timeout
- Batch mode (no interactive prompts)
- Optional custom SSH key file

## Configuration

No environment variables required. The tool uses your system's SSH configuration.

### Optional Settings

```bash
# Default SSH key (can also be passed per-request)
SSH_KEY_PATH=/home/user/.ssh/id_ed25519

# SSH agent socket (if not using default)
SSH_AUTH_SOCK=/run/user/1000/ssh-agent.socket
```

## Usage

### Basic Command Execution

```json
{
  "name": "ssh_exec",
  "arguments": {
    "host": "user@192.168.1.100",
    "command": "uptime"
  }
}
```

### With Custom SSH Key

```json
{
  "name": "ssh_exec",
  "arguments": {
    "host": "admin@homelab.local",
    "command": "docker ps",
    "key": "/home/user/.ssh/homelab_key"
  }
}
```

### With Custom Port

```json
{
  "name": "ssh_exec",
  "arguments": {
    "host": "user@example.com",
    "command": "whoami",
    "port": 2222
  }
}
```

### Complex Command

```json
{
  "name": "ssh_exec",
  "arguments": {
    "host": "root@server.local",
    "command": "df -h / && free -m && cat /etc/os-release"
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `host` | string | Remote host in format `user@hostname` or `hostname` | Yes |
| `command` | string | The command to execute on the remote host | Yes |
| `key` | string | Path to SSH private key file | No |
| `port` | integer | SSH port (default: 22) | No |

## Response Format

Success response:

```json
{
  "content": [
    {
      "type": "text",
      "text": "Output from user@192.168.1.100:\n 10:30:01 up 5 days,  2:30,  1 user,  load average: 0.52, 0.48, 0.44"
    }
  ]
}
```

Error response (command failed):

```json
{
  "content": [
    {
      "type": "text",
      "text": "Command failed (exit code 127) on user@192.168.1.100:\nbash: line 1: nonexistent: command not found"
    }
  ],
  "isError": true
}
```

Error response (SSH connection failed):

```json
{
  "content": [
    {
      "type": "text",
      "text": "Command failed (exit code 255) on user@192.168.1.100:\nssh: connect to host 192.168.1.100 port 22: Connection refused"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: Check Remote Server Status

**Request:**
```json
{
  "name": "ssh_exec",
  "arguments": {
    "host": "admin@webserver.local",
    "command": "systemctl status nginx --no-pager"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Output from admin@webserver.local:\n nginx.service - A high performance web server and a reverse proxy server\n   Loaded: loaded (/lib/systemd/system/nginx.service; enabled)\n   Active: active (running) since Mon 2024-01-15 10:00:00 UTC"
    }
  ]
}
```

### Example 2: Deploy Application

**Request:**
```json
{
  "name": "ssh_exec",
  "arguments": {
    "host": "deploy@app-server.local",
    "command": "cd /opt/myapp && git pull origin main && docker-compose restart"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Output from deploy@app-server.local:\nAlready up to date.\n Container app-web-1  Restarted\n Container app-db-1   Restarted"
    }
  ]
}
```

### Example 3: Check Multiple Servers

**Request (parallel execution via multiple tool calls):**
```json
[
  {
    "name": "ssh_exec",
    "arguments": {
      "host": "node1@cluster.local",
      "command": "hostname && uptime"
    }
  },
  {
    "name": "ssh_exec",
    "arguments": {
      "host": "node2@cluster.local",
      "command": "hostname && uptime"
    }
  }
]
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Output from node1@cluster.local:\nnode1\n 10:30:01 up 12 days,  4:00,  0 users"
    },
    {
      "type": "text",
      "text": "Output from node2@cluster.local:\nnode2\n 10:30:01 up 8 days,  18:30,  0 users"
    }
  ]
}
```

### Example 4: Remote System Diagnostics

**Request:**
```json
{
  "name": "ssh_exec",
  "arguments": {
    "host": "root@problem-server.local",
    "command": "dmesg | tail -20 && journalctl -p err -n 50 --no-pager"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Output from root@problem-server.local:\n[12345.678] Ethernet: link up 1000Mbps full duplex\n[12346.123] systemd[1]: Started Docker application container engine.\n..."
    }
  ]
}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Connection refused" | SSH not running or wrong port | Verify SSH daemon, check port number |
| "Permission denied (publickey)" | Wrong or missing SSH key | Check key path, ensure key is added to ssh-agent |
| "Host key verification failed" | Host key changed | Remove old key from `~/.ssh/known_hosts` |
| "SSH key file not found" | Invalid key path | Verify path exists and is readable |
| "Connection timed out" | Host unreachable or firewall | Check network connectivity, firewall rules |
| "Command not found" | Command doesn't exist on remote | Verify command is in PATH on remote host |
| "Batch mode: no password" | Password auth required but disabled | Set up key-based authentication |

## Security Notes

1. **Key-Based Authentication Only**: This tool uses batch mode which disables password prompts. Always use SSH keys for authentication.

2. **Host Key Verification**: The tool uses `StrictHostKeyChecking=accept-new` which:
   - Accepts new host keys automatically (first connection)
   - Rejects changed host keys (MITM protection)
   - Stores keys in `~/.ssh/known_hosts`

3. **Key File Permissions**: SSH requires private keys to have restrictive permissions:
   ```bash
   chmod 600 ~/.ssh/id_ed25519
   ```

4. **Command Injection**: The command string is passed directly to SSH. Be careful with user-provided input.

5. **Audit Logging**: All SSH commands are logged including the host and command. Review logs for unauthorized access patterns.

6. **SSH Agent Forwarding**: Not enabled by default. If needed for your use case, add `-A` to SSH_OPTS in the script.

7. **Principle of Least Privilege**: Use dedicated SSH keys with minimal permissions. Consider using `command=` restrictions in `authorized_keys` for specific operations.

## SSH Key Setup

### Generate a New Key

```bash
ssh-keygen -t ed25519 -C "jig-mcp@homelab" -f ~/.ssh/jig-mcp_key
```

### Copy to Remote Host

```bash
ssh-copy-id -i ~/.ssh/jig-mcp_key.pub user@remote-host
```

### Add to ssh-agent

```bash
ssh-add ~/.ssh/jig-mcp_key
```

## Files

```
tools/ssh_exec/
├── manifest.yaml       # Tool configuration
├── README.md           # This documentation
└── scripts/
    ├── ssh_exec.sh     # Bash implementation (Linux/macOS)
    └── ssh_exec.ps1    # PowerShell implementation (Windows)
```

## See Also

- [ssh-config(5)](https://man7.org/linux/man-pages/man5/ssh_config.5.html) - SSH client configuration
- [ssh-keygen(1)](https://man7.org/linux/man-pages/man1/ssh-keygen.1.html) - SSH key generation
- [remote_docker_containers](../remote_docker_containers/README.md) - Remote Docker via SSH
- [remote_docker_compose](../remote_docker_compose/README.md) - Remote Compose via SSH
