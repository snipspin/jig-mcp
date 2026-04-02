#!/bin/bash
set -euo pipefail

JSON_ARGS="$1"

# Parse arguments
action=$(echo "$JSON_ARGS" | sed -n 's/.*"action"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
container=$(echo "$JSON_ARGS" | sed -n 's/.*"container"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
host=$(echo "$JSON_ARGS" | sed -n 's/.*"host"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
key=$(echo "$JSON_ARGS" | sed -n 's/.*"key"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
port=$(echo "$JSON_ARGS" | sed -n 's/.*"port"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')
tail_lines=$(echo "$JSON_ARGS" | sed -n 's/.*"tail"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')
show_all=$(echo "$JSON_ARGS" | sed -n 's/.*"all"[[:space:]]*:[[:space:]]*\(true\|false\).*/\1/p')

port=${port:-22}
tail_lines=${tail_lines:-100}

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

# Validate container param for actions that need it
require_container() {
    if [ -z "$container" ]; then
        send_response "Error: 'container' parameter is required for action '$action'" "true"
    fi
}

case "$action" in
    list)
        if [ "$show_all" = "true" ]; then
            result=$(ssh $SSH_OPTS "$host" "docker ps -a" 2>&1) || send_response "Error listing containers: $result" "true"
        else
            result=$(ssh $SSH_OPTS "$host" "docker ps" 2>&1) || send_response "Error listing containers: $result" "true"
        fi
        send_response "$result"
        ;;
    start)
        require_container
        result=$(ssh $SSH_OPTS "$host" "docker start '$container'" 2>&1) || send_response "Error starting container: $result" "true"
        send_response "Container '$container' started successfully."
        ;;
    stop)
        require_container
        result=$(ssh $SSH_OPTS "$host" "docker stop '$container'" 2>&1) || send_response "Error stopping container: $result" "true"
        send_response "Container '$container' stopped successfully."
        ;;
    restart)
        require_container
        result=$(ssh $SSH_OPTS "$host" "docker restart '$container'" 2>&1) || send_response "Error restarting container: $result" "true"
        send_response "Container '$container' restarted successfully."
        ;;
    logs)
        require_container
        result=$(ssh $SSH_OPTS "$host" "docker logs --tail '$tail_lines' '$container'" 2>&1) || send_response "Error fetching logs: $result" "true"
        send_response "$result"
        ;;
    inspect)
        require_container
        result=$(ssh $SSH_OPTS "$host" "docker inspect '$container'" 2>&1) || send_response "Error inspecting container: $result" "true"
        send_response "$result"
        ;;
    remove)
        require_container
        result=$(ssh $SSH_OPTS "$host" "docker rm -f '$container'" 2>&1) || send_response "Error removing container: $result" "true"
        send_response "Container '$container' removed successfully."
        ;;
    *)
        send_response "Unknown action: '$action'. Valid actions: list, start, stop, restart, logs, inspect, remove" "true"
        ;;
esac
