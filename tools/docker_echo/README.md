# docker_echo

Docker connectivity check that runs `echo` inside an Alpine container. Verifies Docker is installed and functioning correctly with sandboxed execution.

## Overview

`docker_echo` is a minimal diagnostic tool that:

- **Tests Docker installation** - Verifies Docker daemon is running
- **Validates sandbox execution** - Confirms Docker sandbox mode works
- **Returns predictable output** - Always returns "hello from alpine sandbox"
- **Cross-platform** - Works on Linux, macOS, and Windows

### Use Cases

- Verify Docker installation after setup
- Test Docker sandbox functionality before deploying untrusted tools
- Debug Docker connectivity issues
- Validate jig-mcp Docker integration
- Quick health check for Docker-dependent workflows

### How It Works

The tool executes a simple `echo` command inside an Alpine Linux container:

```bash
docker run --rm alpine:latest echo "hello from alpine sandbox"
```

This confirms:
1. Docker daemon is running
2. Images can be pulled (if not cached)
3. Containers can be created and executed
4. Output is captured and returned

## Configuration

No environment variables required.

### Requirements

- Docker installed and running
- Docker daemon accessible to the jig-mcp process
- Network access to pull `alpine:latest` (if not cached)

## Usage

### Basic Connectivity Test

```json
{
  "name": "docker_echo",
  "arguments": {}
}
```

That's it - no parameters needed. The tool runs the echo command and returns the output.

### Input Parameters

This tool takes no input parameters.

## Response Format

### Success Response

```json
{
  "content": [
    {
      "type": "text",
      "text": "hello from alpine sandbox"
    }
  ]
}
```

### Error Response (Docker Not Running)

```json
{
  "content": [
    {
      "type": "text",
      "text": "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?"
    }
  ],
  "isError": true
}
```

### Error Response (Image Pull Failed)

```json
{
  "content": [
    {
      "type": "text",
      "text": "Unable to find image 'alpine:latest' locally\ndocker: Error response from daemon: Get https://registry-1.docker.io/v2/: dial tcp: lookup registry-1.docker.io: no such host"
    }
  ],
  "isError": true
}
```

## Examples

### Example 1: Verify Docker Installation

**Request:**
```json
{
  "name": "docker_echo",
  "arguments": {}
}
```

**Response (First Run - Pulling Image):**
```json
{
  "content": [
    {
      "type": "text",
      "text": "Unable to find image 'alpine:latest' locally\nlatest: Pulling from library/alpine\ncd782f6a1ab7: Pull complete\nDigest: sha256:abcd1234...\nStatus: Downloaded newer image for alpine:latest\nhello from alpine sandbox"
    }
  ]
}
```

**Response (Subsequent Runs - Cached Image):**
```json
{
  "content": [
    {
      "type": "text",
      "text": "hello from alpine sandbox"
    }
  ]
}
```

### Example 2: Pre-Flight Docker Check

Before running Docker-dependent tools, verify Docker is working:

```json
{"name": "docker_echo", "arguments": {}}
```

If successful, proceed with Docker operations. If failed, diagnose Docker issues first.

### Example 3: Test Sandbox Mode

The `docker_echo` tool uses Docker sandbox mode defined in its manifest:

```yaml
sandbox:
  type: docker
  image: alpine:latest
```

This confirms that sandboxed tool execution is working before deploying untrusted tools.

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| "Cannot connect to Docker daemon" | Docker not running | Start Docker: `sudo systemctl start docker` |
| "permission denied" | User not in docker group | Add user: `sudo usermod -aG docker $USER` |
| "Unable to find image" | Image not cached, no network | Check network connectivity, proxy settings |
| "context deadline exceeded" | Docker daemon slow or unresponsive | Restart Docker daemon, check system resources |
| "no such host" | DNS resolution failed | Check DNS settings, network connectivity |

## Security Notes

1. **Sandboxed Execution**: This tool runs inside a Docker container, providing isolation from the host system.

2. **Minimal Privileges**: The container runs with default (non-privileged) settings and is removed after execution (`--rm`).

3. **No Network Exposure**: The container doesn't expose any ports or services.

4. **Read-Only Operation**: The tool only runs `echo` - no file system modifications.

5. **Image Verification**: The `alpine:latest` image is pulled from Docker Hub. For production use, consider pinning to a specific digest:
   ```yaml
   sandbox:
     image: alpine@sha256:abcd1234...
   ```

## Docker Setup

### Install Docker (Linux)

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install docker.io docker-compose

# Start Docker
sudo systemctl enable --now docker

# Add user to docker group
sudo usermod -aG docker $USER
# Log out and back in for group changes to take effect
```

### Install Docker (macOS)

Install Docker Desktop from https://www.docker.com/products/docker-desktop/

### Install Docker (Windows)

Install Docker Desktop from https://www.docker.com/products/docker-desktop/

### Verify Installation

```bash
docker run --rm hello-world
```

## Files

```
tools/docker_echo/
├── manifest.yaml       # Tool configuration with sandbox settings
├── README.md           # This documentation
└── scripts/            # (No scripts - uses Docker sandbox directly)
```

## See Also

- [hello](../hello/README.md) - Basic example tool
- [docker_containers](../docker_containers/README.md) - Docker container management
- [Docker Installation Guide](https://docs.docker.com/engine/install/) - Official docs
- [Docker Sandbox](../../docs/TOOL_DEVELOPER_GUIDE.md#sandboxed-execution) - Sandboxed tool execution
