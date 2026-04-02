#!/bin/bash
set -euo pipefail

JSON_ARGS="$1"

# Parse arguments
action=$(echo "$JSON_ARGS" | sed -n 's/.*"action"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
image=$(echo "$JSON_ARGS" | sed -n 's/.*"image"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
host=$(echo "$JSON_ARGS" | sed -n 's/.*"host"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
key=$(echo "$JSON_ARGS" | sed -n 's/.*"key"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
port=$(echo "$JSON_ARGS" | sed -n 's/.*"port"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')
show_all=$(echo "$JSON_ARGS" | sed -n 's/.*"all"[[:space:]]*:[[:space:]]*\(true\|false\).*/\1/p')

port=${port:-22}

send_response() {
    local text="$1"
    local is_error="${2:-false}"
    local escaped
    escaped=$(printf '%s' "$text" | sed 's/\\/\\\\/g; s/"/\\"/g' | awk '{printf "%s\\n", $0}' | sed '$ s/\\n$//')
    if [ "$is_error" = "true" ]; then
        printf '{"content":[{"type":"text","text":"%s"}],"isError":true}\n' "$escaped"
        exit 1
    else
        printf '{"content":[{"type":"text","text":"%s"}]}\n' "$escaped"
    fi
}

if [ -z "$host" ]; then
    send_response "Error: 'host' parameter is required." "true"
fi

# Build SSH command
SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -o BatchMode=yes -p $port"
if [ -n "$key" ]; then
    if [ ! -f "$key" ]; then
        send_response "Error: SSH key file not found: $key" "true"
    fi
    SSH_OPTS="$SSH_OPTS -i $key"
fi

case "$action" in
    list)
        if [ "$show_all" = "true" ]; then
            result=$(ssh $SSH_OPTS "$host" "docker images -a" 2>&1) || send_response "Error listing images: $result" "true"
        else
            result=$(ssh $SSH_OPTS "$host" "docker images" 2>&1) || send_response "Error listing images: $result" "true"
        fi
        send_response "$result"
        ;;
    pull)
        if [ -z "$image" ]; then
            send_response "Error: 'image' parameter is required for 'pull' action." "true"
        fi
        result=$(ssh $SSH_OPTS "$host" "docker pull '$image'" 2>&1) || send_response "Error pulling image: $result" "true"
        send_response "Image '$image' pulled successfully.\n$result"
        ;;
    remove)
        if [ -z "$image" ]; then
            send_response "Error: 'image' parameter is required for 'remove' action." "true"
        fi
        result=$(ssh $SSH_OPTS "$host" "docker rmi '$image'" 2>&1) || send_response "Error removing image: $result" "true"
        send_response "Image '$image' removed successfully.\n$result"
        ;;
    prune)
        flags="-f"
        if [ "$show_all" = "true" ]; then
            flags="-a -f"
        fi
        result=$(ssh $SSH_OPTS "$host" "docker image prune $flags" 2>&1) || send_response "Error pruning images: $result" "true"
        send_response "Image prune completed.\n$result"
        ;;
    *)
        send_response "Unknown action: '$action'. Valid actions: list, pull, remove, prune" "true"
        ;;
esac
