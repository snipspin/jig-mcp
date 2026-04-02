#!/bin/bash
set -euo pipefail

JSON_ARGS="$1"

action=$(echo "$JSON_ARGS" | sed -n 's/.*"action"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
project_dir=$(echo "$JSON_ARGS" | sed -n 's/.*"project_dir"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
host=$(echo "$JSON_ARGS" | sed -n 's/.*"host"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
service=$(echo "$JSON_ARGS" | sed -n 's/.*"service"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
tail_lines=$(echo "$JSON_ARGS" | sed -n 's/.*"tail"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')

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

if [ -z "$project_dir" ]; then
    send_response "Error: 'project_dir' parameter is required." "true"
fi

# Build base command
DOCKER_CMD="docker"
if [ -n "$host" ]; then
    DOCKER_CMD="docker -H $host"
fi
COMPOSE_CMD="$DOCKER_CMD compose -f $project_dir/docker-compose.yml"

# Append service if specified
svc_arg=""
if [ -n "$service" ]; then
    svc_arg="$service"
fi

case "$action" in
    up)
        result=$($COMPOSE_CMD up -d $svc_arg 2>&1) || send_response "Error starting stack: $result" "true"
        send_response "Stack in '$project_dir' started successfully.\n$result"
        ;;
    down)
        result=$($COMPOSE_CMD down 2>&1) || send_response "Error stopping stack: $result" "true"
        send_response "Stack in '$project_dir' stopped and removed.\n$result"
        ;;
    ps)
        result=$($COMPOSE_CMD ps 2>&1) || send_response "Error listing services: $result" "true"
        send_response "$result"
        ;;
    logs)
        result=$($COMPOSE_CMD logs --tail "$tail_lines" $svc_arg 2>&1) || send_response "Error fetching logs: $result" "true"
        send_response "$result"
        ;;
    pull)
        result=$($COMPOSE_CMD pull $svc_arg 2>&1) || send_response "Error pulling images: $result" "true"
        send_response "Images pulled for stack in '$project_dir'.\n$result"
        ;;
    restart)
        result=$($COMPOSE_CMD restart $svc_arg 2>&1) || send_response "Error restarting stack: $result" "true"
        send_response "Stack in '$project_dir' restarted.\n$result"
        ;;
    *)
        send_response "Unknown action: '$action'. Valid actions: up, down, ps, logs, pull, restart" "true"
        ;;
esac
