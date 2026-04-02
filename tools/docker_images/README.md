# docker_images

Manage Docker images: list, pull, remove, and prune unused images. Supports both local and remote Docker hosts via SSH.

## Overview

The `docker_images` tool provides complete Docker image management:

- **List images** - View all images with size and creation info
- **Pull images** - Download images from registries
- **Remove images** - Delete images from the system
- **Prune images** - Clean up dangling or all unused images
- **Remote support** - Manage images on remote Docker hosts

### Use Cases

- Inventory available images
- Pull new images for deployments
- Clean up disk space by removing old images
- Prune unused images after builds
- Manage images on remote Docker hosts

## Configuration

No environment variables required. The tool uses the local Docker socket by default.

### Optional Settings

```bash
# Remote Docker host (can also be passed per-request)
DOCKER_HOST=ssh://user@remote-server

# Docker socket path (for non-default locations)
DOCKER_SOCKET=/var/run/docker.sock

# Docker registry credentials (for private registries)
DOCKER_REGISTRY_USER=username
DOCKER_REGISTRY_PASS=password
```

## Usage

### List All Images

```json
{
  "name": "docker_images",
  "arguments": {
    "action": "list"
  }
}
```

### List All Images (Including Intermediate Layers)

```json
{
  "name": "docker_images",
  "arguments": {
    "action": "list",
    "all": true
  }
}
```

### Pull an Image

```json
{
  "name": "docker_images",
  "arguments": {
    "action": "pull",
    "image": "nginx:latest"
  }
}
```

### Remove an Image

```json
{
  "name": "docker_images",
  "arguments": {
    "action": "remove",
    "image": "old-image:tag"
  }
}
```

### Prune Dangling Images

```json
{
  "name": "docker_images",
  "arguments": {
    "action": "prune"
  }
}
```

### Prune All Unused Images

```json
{
  "name": "docker_images",
  "arguments": {
    "action": "prune",
    "all": true
  }
}
```

### Remote Docker Host

```json
{
  "name": "docker_images",
  "arguments": {
    "action": "list",
    "host": "ssh://user@192.168.1.100"
  }
}
```

### Input Parameters

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| `action` | string | Action: list, pull, remove, prune | Yes |
| `image` | string | Image name with optional tag | Yes (pull, remove) |
| `host` | string | Remote Docker host (e.g., `ssh://user@server`) | No |
| `all` | boolean | List intermediate images / Prune all unused | No |

## Response Format

### List Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "REPOSITORY       TAG       IMAGE ID       SIZE        CREATED\nnginx            latest      abc123def456   187MB       2 weeks ago\nredis            alpine      def789ghi012   35.2MB      3 weeks ago\nnode             18          123abc456def   1.12GB      1 month ago"
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
      "text": "Error: No such image: nonexistent-image"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: List Available Images

**Request:**
```json
{
  "name": "docker_images",
  "arguments": {
    "action": "list"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "REPOSITORY          TAG       IMAGE ID       SIZE        CREATED
nginx               latest      abc123def456   187MB       2 weeks ago\nredis               alpine      def789ghi012   35.2MB      3 weeks ago\npostgres            15          456def789abc   431MB       1 month ago\nnode                18-slim     789abc123def   245MB       1 month ago"
    }
  ]
}
```

### Example 2: Pull New Image for Deployment

**Request:**
```json
{
  "name": "docker_images",
  "arguments": {
    "action": "pull",
    "image": "myapp:v2.0.0"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Image 'myapp:v2.0.0' pulled successfully.\nlatest: Pulling from library/myapp\nabc123def456: Pull complete\ndef789ghi012: Pull complete\nDigest: sha256:xyz789...\nStatus: Downloaded newer image for myapp:v2.0.0"
    }
  ]
}
```

### Example 3: Clean Up Disk Space

**Request:**
```json
{
  "name": "docker_images",
  "arguments": {
    "action": "prune",
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
      "text": "Image prune completed.\nDeleted Images:\nuntagged: node:16.0.0\nuntagged: node:16.1.0\nuntagged: python:3.9\ndeleted: sha256:abc123...\ndeleted: sha256:def456...\nTotal reclaimed space: 1.2GB"
    }
  ]
}
```

### Example 4: Remove Specific Old Image

**Request:**
```json
{
  "name": "docker_images",
  "arguments": {
    "action": "remove",
    "image": "myapp:v1.0.0"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Image 'myapp:v1.0.0' removed successfully.\nUntagged: myapp:v1.0.0\nDeleted: sha256:old123..."
    }
  ]
}
```

### Example 5: Remote Image Management

**Request:**
```json
{
  "name": "docker_images",
  "arguments": {
    "action": "list",
    "host": "ssh://admin@build-server.local"
  }
}
```

**Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "REPOSITORY          TAG       IMAGE ID       SIZE        CREATED\nmyapp               build-42  abc123def456   256MB       1 hour ago\nmyapp               build-41  def789ghi012   254MB       2 hours ago\ngolang              1.22      456def789abc   912MB       1 day ago"
    }
  ]
}
```

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Cannot connect to Docker daemon" | Docker not running or socket inaccessible | Start Docker service, check socket permissions |
| "No such image" | Image doesn't exist or wrong tag | Use `action: list` to verify image name |
| "Image is being used by running container" | Can't remove in-use images | Stop/remove container first, or use `-f` |
| "Permission denied" | SSH authentication failed | Check SSH key, verify host format |
| "Pull access denied" | Registry authentication required | Configure registry credentials, check image name |
| "No space left on device" | Disk full, can't pull new images | Run `prune` to free space |

## Security Notes

1. **Docker Socket Access**: Access to the Docker socket provides root-equivalent privileges. Only trusted users should access jig-mcp when this tool is enabled.

2. **Image Removal**: Removing images can break running containers that depend on them. Verify no containers are using an image before removal.

3. **Prune Caution**: `prune` with `all: true` removes ALL unused images, not just dangling ones. This can free significant space but requires repulling for future use.

4. **Remote Host Security**: When using remote Docker hosts:
   - Use SSH key authentication
   - Restrict SSH keys to specific commands if possible
   - Prefer TLS for TCP connections

5. **Registry Credentials**: If using private registries, store credentials securely and never commit them to version control.

6. **Audit Logging**: All image operations are logged. Review logs for unauthorized access attempts.

## Files

```
tools/docker_images/
├── manifest.yaml           # Tool configuration
├── README.md               # This documentation
└── scripts/
    ├── docker_images.sh    # Bash implementation (Linux/macOS)
    └── docker_images.ps1   # PowerShell implementation (Windows)
```

## See Also

- [docker_containers](../docker_containers/README.md) - Manage Docker containers
- [docker_compose](../docker_compose/README.md) - Multi-container orchestration
- [Docker Images Reference](https://docs.docker.com/engine/reference/commandline/image/) - Official docs
- [Docker Prune](https://docs.docker.com/config/pruning/) - Cleanup documentation
