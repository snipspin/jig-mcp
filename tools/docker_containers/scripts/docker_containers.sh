#!/bin/bash
set -euo pipefail

JSON_ARGS="$1"

# Parse arguments (no jq dependency)
action=$(echo "$JSON_ARGS" | sed -n 's/.*"action"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
container=$(echo "$JSON_ARGS" | sed -n 's/.*"container"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
host=$(echo "$JSON_ARGS" | sed -n 's/.*"host"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
tail_lines=$(echo "$JSON_ARGS" | sed -n 's/.*"tail"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')
show_all=$(echo "$JSON_ARGS" | sed -n 's/.*"all"[[:space:]]*:[[:space:]]*\(true\|false\).*/\1/p')

tail_lines=${tail_lines:-100}

send_response() {
    local text="$1"
    local is_error="${2:-false}"
    # Escape for JSON: backslashes, double quotes, newlines, tabs, carriage returns
    local escaped
    escaped=$(printf '%s' "$text" | sed 's/\\/\\\\/g; s/"/\\"/g' | awk '{printf "%s\\n", $0}' | sed '$ s/\\n$//')
    if [ "$is_error" = "true" ]; then
        printf '{"content":[{"type":"text","text":"%s"}],"isError":true}\n' "$escaped"
        exit 1
    else
        printf '{"content":[{"type":"text","text":"%s"}]}\n' "$escaped"
    fi
}

# Build docker command with optional remote host
DOCKER_CMD="docker"
if [ -n "$host" ]; then
    DOCKER_CMD="docker -H $host"
fi

# Validate container param for actions that need it
require_container() {
    if [ -z "$container" ]; then
        send_response "Error: 'container' parameter is required for action '$action'" "true"
    fi
}

case "$action" in
    list)
        flags="--format table {{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
        if [ "$show_all" = "true" ]; then
            flags="-a $flags"
        fi
        result=$($DOCKER_CMD ps $flags 2>&1) || send_response "Error listing containers: $result" "true"
        send_response "$result"
        ;;
    start)
        require_container
        result=$($DOCKER_CMD start "$container" 2>&1) || send_response "Error starting container: $result" "true"
        send_response "Container '$container' started successfully."
        ;;
    stop)
        require_container
        result=$($DOCKER_CMD stop "$container" 2>&1) || send_response "Error stopping container: $result" "true"
        send_response "Container '$container' stopped successfully."
        ;;
    restart)
        require_container
        result=$($DOCKER_CMD restart "$container" 2>&1) || send_response "Error restarting container: $result" "true"
        send_response "Container '$container' restarted successfully."
        ;;
    logs)
        require_container
        result=$($DOCKER_CMD logs --tail "$tail_lines" "$container" 2>&1) || send_response "Error fetching logs: $result" "true"
        send_response "$result"
        ;;
    inspect)
        require_container
        result=$($DOCKER_CMD inspect "$container" 2>&1) || send_response "Error inspecting container: $result" "true"
        send_response "$result"
        ;;
    remove)
        require_container
        result=$($DOCKER_CMD rm -f "$container" 2>&1) || send_response "Error removing container: $result" "true"
        send_response "Container '$container' removed successfully."
        ;;
    *)
        send_response "Unknown action: '$action'. Valid actions: list, start, stop, restart, logs, inspect, remove" "true"
        ;;
esac
