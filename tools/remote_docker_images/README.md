# remote_docker_images

Manage Docker images on remote hosts via SSH: list, pull, remove, and prune unused images. Combines SSH key-based authentication with Docker CLI for remote image management.

## Overview

`remote_docker_images` provides complete Docker image management on remote hosts:

- **All image operations** - list, pull, remove, prune
- **SSH key authentication** - Secure, passwordless remote access
- **Custom SSH ports** - Support for non-standard SSH configurations
- **Prune modes** - Dangling-only or all unused images
- **Platform support** - Bash on Linux/macOS, PowerShell on Windows

### Use Cases

- Inventory images on remote Docker hosts
- Pull new images before deployment
- Clean up disk space by removing old images
- Prune unused images after builds
- Manage images on homelab servers

### How It Works

The tool SSHs into the remote host and executes Docker CLI commands:
```bash
ssh user@host "docker images"
ssh user@host "docker pull nginx:latest"
ssh user@host "docker image prune -f"
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

### List All Images

```json
{
  "name": "remote_docker_images",
  "arguments": {
    "action": "list",
    "host": "user@192.168.1.100"
  }
}
```

### List All Images (Including Intermediate Layers)

```json
{
  "name": "remote_docker_images",
  "arguments": {
    "action": "list",
    "host": "admin@homelab.local",
    "all": true
  }
}
```

### Pull an Image

```json
{
  "name": "remote_docker_images",
  "arguments": {
    "action": "pull",
    "host": "admin@homelab.local",
    "image": "nginx:latest"
  }
}
```

### Remove an Image

```json
{
  "name": "remote_docker_images",
  "arguments": {
    "action": "remove",
    "host": "admin@homelab.local",
    "image": "old-image:tag"
  }
}
```

### Prune Dangling Images

```json
{
  "name": "remote_docker_images",
  "arguments": {
    "action": "prune",
    "host": "admin@homelab.local"
  }
}
```

### Prune All Unused Images

```json
{
  "name": "remote_docker_images",
  "arguments": {
    "action": "prune",
    "host": "admin@homelab.local",
    "all": true
  }
}
```

### With Custom SSH Key

```json
{
  "name": "remote_docker_images",
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
  "name": "remote_docker_images",
  "arguments": {
    "action": "pull",
    "host": "user@server.local",
    "port": 2222,
    "image": "alpine:latest"
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `action` | string | Action: list, pull, remove, prune | Yes |
| `host` | string | Remote host in format `user@hostname` or `hostname` | Yes |
| `image` | string | Image name with optional tag | Yes (pull, remove) |
| `key` | string | Path to SSH private key file | No |
| `port` | integer | SSH port (default: 22) | No |
| `all` | boolean | List intermediate / Prune all unused | No |

## Response Format

### List Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "REPOSITORY       TAG       IMAGE ID       SIZE        CREATED\nnginx            latest      abc123def456   187MB       2 weeks ago\nredis            alpine      def789ghi012   35.2MB      3 weeks ago"
    }
  ]
}
```

### Pull Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Image 'nginx:latest' pulled successfully.\nlatest: Pulling from library/nginx\nabc123def456: Pull complete\ndef789ghi012: Pull complete\nDigest: sha256:abcd1234...\nStatus: Downloaded newer image for nginx:latest"
    }
  ]
}
```

### Remove Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Image 'old-image:tag' removed successfully.\nUntagged: old-image:tag\nDeleted: sha256:abc123..."
    }
  ]
}
```

### Prune Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "Image prune completed.\nDeleted Images:\nuntagged: nginx:1.19\nuntagged: nginx:1.19-alpine\ndeleted: sha256:abc123...\nTotal reclaimed space: 156.7MB"
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
      "text": "Error pulling image: No such image: nonexistent:latest"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: List Images on Homelab Server

**Request:**
```json
{
  "name": "remote_docker_images",
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
      "text": "REPOSITORY          TAG       IMAGE ID       SIZE        CREATED\npihole/pihole       latest      abc123def456   75MB        1 month ago\nnextcloud           latest      def789ghi012   520MB       2 weeks ago\nhome-assistant      latest      456def789abc   1.2GB       3 weeks ago\nlinuxserver/plex    latest      789abc123def   450MB       1 week ago"
    }
  ]
}
```

### Example 2: Pull Updated Image Before Restart

**Request:**
```json
{
  "name": "remote_docker_images",
  "arguments": {
    "action": "pull",
    "host": "admin@homelab.local",
    "image": "pihole/pihole:latest"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Image 'pihole/pihole:latest' pulled successfully.\nlatest: Pulling from pihole/pihole\nabc123def456: Pull complete\ndef789ghi012: Pull complete\nDigest: sha256:xyz789...\nStatus: Downloaded newer image for pihole/pihole:latest"
    }
  ]
}
```

### Example 3: Clean Up Disk Space After Builds

**Request:**
```json
{
  "name": "remote_docker_images",
  "arguments": {
    "action": "prune",
    "host": "admin@build-server.local",
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
      "text": "Image prune completed.\nDeleted Images:\nuntagged: myapp:build-40\nuntagged: myapp:build-41\nuntagged: golang:1.21\ndeleted: sha256:abc123...\ndeleted: sha256:def456...\nTotal reclaimed space: 2.1GB"
    }
  ]
}
```

### Example 4: Remove Specific Old Image

**Request:**
```json
{
  "name": "remote_docker_images",
  "arguments": {
    "action": "remove",
    "host": "admin@homelab.local",
    "image": "old-backup:latest"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Image 'old-backup:latest' removed successfully.\nUntagged: old-backup:latest\nDeleted: sha256:old123..."
    }
  ]
}
```

### Example 5: Check Images on Multiple Hosts

**Request (parallel execution):**
```json
[
  {
    "name": "remote_docker_images",
    "arguments": {
      "action": "list",
      "host": "node1@cluster.local"
    }
  },
  {
    "name": "remote_docker_images",
    "arguments": {
      "action": "list",
      "host": "node2@cluster.local"
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
      "text": "REPOSITORY    TAG    IMAGE ID       SIZE      CREATED\nnode1-app     v2.0   abc123def456   256MB     1 day ago\nnode1-db      15     def789ghi012   431MB     1 week ago"
    },
    {
      "type": "text",
      "text": "REPOSITORY    TAG    IMAGE ID       SIZE      CREATED\nnode2-app     v2.0   abc123def456   256MB     1 day ago\nnode2-cache   7      456def789abc   35MB      2 weeks ago"
    }
  ]
}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Cannot connect to Docker daemon" | Docker not running on remote host | Start Docker: `sudo systemctl start docker` |
| "Permission denied" | SSH authentication failed | Check SSH key, verify authorized_keys |
| "Got permission denied while trying to connect to the Docker daemon socket" | User not in docker group | Add user to docker group on remote host |
| "No such image" | Image doesn't exist | Use `action: list` to verify image name |
| "Image is being used by running container" | Can't remove in-use images | Stop/remove container first |
| "Connection refused" | SSH not running or wrong port | Verify SSH daemon, check port number |
| "No space left on device" | Disk full, can't pull new images | Run `prune` to free space first |

## Security Notes

1. **SSH Key Security**: Use dedicated SSH keys for jig-mcp. Store keys securely and never commit to version control.

2. **Docker Group Membership**: Remote users must be in the `docker` group for passwordless Docker access. This provides root-equivalent privileges - only trust users with SSH access.

3. **Image Removal**: Removing images can break running containers that depend on them. Verify no containers are using an image before removal.

4. **Prune Caution**: `prune` with `all: true` removes ALL unused images, not just dangling ones. This can free significant space but requires repulling for future use.

5. **Host Key Verification**: Uses `StrictHostKeyChecking=accept-new`:
   - Accepts new keys on first connection
   - Rejects changed keys (MITM protection)

6. **Audit Logging**: All image operations are logged with host, action, and image name. Review logs for unauthorized access patterns.

## SSH Key Setup for Remote Image Management

### 1. Generate SSH Key

```bash
ssh-keygen -t ed25519 -C "jig-mcp-images" -f ~/.ssh/jig-mcp-images
```

### 2. Copy to Remote Host

```bash
ssh-copy-id -i ~/.ssh/jig-mcp-images.pub user@remote-host
```

### 3. Add User to Docker Group (on remote host)

```bash
ssh user@remote-host "sudo usermod -aG docker $USER"
```

### 4. Test Connection

```bash
ssh -i ~/.ssh/jig-mcp-images user@remote-host "docker images"
```

## Files

```
tools/remote_docker_images/
├── manifest.yaml                   # Tool configuration
├── README.md                       # This documentation
└── scripts/
    ├── remote_docker_images.sh     # Bash implementation (Linux/macOS)
    └── remote_docker_images.ps1    # PowerShell implementation (Windows)
```

## See Also

- [docker_images](../docker_images/README.md) - Local Docker image management
- [ssh_exec](../ssh_exec/README.md) - Generic SSH command execution
- [remote_docker_containers](../remote_docker_containers/README.md) - Remote container management
- [Docker Images Reference](https://docs.docker.com/engine/reference/commandline/image/) - Official docs
