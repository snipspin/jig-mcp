#!/bin/bash
set -euo pipefail

JSON_ARGS="$1"

host=$(echo "$JSON_ARGS" | sed -n 's/.*"host"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
command_str=$(echo "$JSON_ARGS" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
key=$(echo "$JSON_ARGS" | sed -n 's/.*"key"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
port=$(echo "$JSON_ARGS" | sed -n 's/.*"port"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')

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
if [ -z "$command_str" ]; then
    send_response "Error: 'command' parameter is required." "true"
fi

# Build SSH command
SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -o BatchMode=yes -p $port"
if [ -n "$key" ]; then
    if [ ! -f "$key" ]; then
        send_response "Error: SSH key file not found: $key" "true"
    fi
    SSH_OPTS="$SSH_OPTS -i $key"
fi

result=$(ssh $SSH_OPTS "$host" "$command_str" 2>&1) || {
    exit_code=$?
    send_response "Command failed (exit code $exit_code) on $host:\n$result" "true"
}

send_response "Output from $host:\n$result"
