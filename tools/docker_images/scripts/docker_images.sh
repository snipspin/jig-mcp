#!/bin/bash
set -euo pipefail

JSON_ARGS="$1"

action=$(echo "$JSON_ARGS" | sed -n 's/.*"action"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
image=$(echo "$JSON_ARGS" | sed -n 's/.*"image"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
host=$(echo "$JSON_ARGS" | sed -n 's/.*"host"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
show_all=$(echo "$JSON_ARGS" | sed -n 's/.*"all"[[:space:]]*:[[:space:]]*\(true\|false\).*/\1/p')

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

DOCKER_CMD="docker"
if [ -n "$host" ]; then
    DOCKER_CMD="docker -H $host"
fi

case "$action" in
    list)
        flags="--format table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}\t{{.CreatedSince}}"
        if [ "$show_all" = "true" ]; then
            flags="-a $flags"
        fi
        result=$($DOCKER_CMD images $flags 2>&1) || send_response "Error listing images: $result" "true"
        send_response "$result"
        ;;
    pull)
        if [ -z "$image" ]; then
            send_response "Error: 'image' parameter is required for 'pull' action." "true"
        fi
        result=$($DOCKER_CMD pull "$image" 2>&1) || send_response "Error pulling image: $result" "true"
        send_response "Image '$image' pulled successfully.\n$result"
        ;;
    remove)
        if [ -z "$image" ]; then
            send_response "Error: 'image' parameter is required for 'remove' action." "true"
        fi
        result=$($DOCKER_CMD rmi "$image" 2>&1) || send_response "Error removing image: $result" "true"
        send_response "Image '$image' removed successfully.\n$result"
        ;;
    prune)
        flags="-f"
        if [ "$show_all" = "true" ]; then
            flags="-a -f"
        fi
        result=$($DOCKER_CMD image prune $flags 2>&1) || send_response "Error pruning images: $result" "true"
        send_response "Image prune completed.\n$result"
        ;;
    *)
        send_response "Unknown action: '$action'. Valid actions: list, pull, remove, prune" "true"
        ;;
esac
