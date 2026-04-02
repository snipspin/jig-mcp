#!/bin/bash
set -euo pipefail

JSON_ARGS="$1"

check=$(echo "$JSON_ARGS" | sed -n 's/.*"check"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
target=$(echo "$JSON_ARGS" | sed -n 's/.*"target"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
port=$(echo "$JSON_ARGS" | sed -n 's/.*"port"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')
path=$(echo "$JSON_ARGS" | sed -n 's/.*"path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
scheme=$(echo "$JSON_ARGS" | sed -n 's/.*"scheme"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
timeout_sec=$(echo "$JSON_ARGS" | sed -n 's/.*"timeout"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')

timeout_sec=${timeout_sec:-5}
path=${path:-/}
scheme=${scheme:-http}

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

if [ -z "$target" ]; then
    send_response "Error: 'target' parameter is required." "true"
fi

case "$check" in
    ping)
        # Use -c 3 for 3 pings, -W for timeout
        if ping -c 3 -W "$timeout_sec" "$target" > /tmp/jig_ping_$$ 2>&1; then
            result=$(cat /tmp/jig_ping_$$)
            rm -f /tmp/jig_ping_$$
            send_response "HEALTHY: $target is reachable.\n\n$result"
        else
            result=$(cat /tmp/jig_ping_$$ 2>/dev/null || echo "No response")
            rm -f /tmp/jig_ping_$$
            send_response "UNHEALTHY: $target is not reachable.\n\n$result" "true"
        fi
        ;;
    port)
        if [ -z "$port" ]; then
            send_response "Error: 'port' parameter is required for port check." "true"
        fi
        # Try TCP connection using /dev/tcp or nc
        if command -v nc &>/dev/null; then
            if nc -z -w "$timeout_sec" "$target" "$port" 2>&1; then
                send_response "HEALTHY: $target:$port is open and accepting connections."
            else
                send_response "UNHEALTHY: $target:$port is not reachable or refused connection." "true"
            fi
        elif (echo >/dev/tcp/"$target"/"$port") 2>/dev/null; then
            send_response "HEALTHY: $target:$port is open and accepting connections."
        else
            send_response "UNHEALTHY: $target:$port is not reachable or refused connection." "true"
        fi
        ;;
    http)
        url="${scheme}://${target}"
        if [ -n "$port" ]; then
            url="${scheme}://${target}:${port}"
        fi
        url="${url}${path}"

        if command -v curl &>/dev/null; then
            http_code=$(curl -s -o /tmp/jig_http_$$ -w '%{http_code}' --connect-timeout "$timeout_sec" --max-time "$((timeout_sec * 2))" "$url" 2>&1) || true
            body=$(cat /tmp/jig_http_$$ 2>/dev/null | head -c 2048 || echo "")
            rm -f /tmp/jig_http_$$

            if [ -z "$http_code" ] || [ "$http_code" = "000" ]; then
                send_response "UNHEALTHY: Could not connect to $url (connection failed or timed out)." "true"
            elif [ "$http_code" -ge 200 ] && [ "$http_code" -lt 400 ]; then
                send_response "HEALTHY: $url returned HTTP $http_code.\n\nResponse (truncated):\n$body"
            else
                send_response "UNHEALTHY: $url returned HTTP $http_code.\n\nResponse (truncated):\n$body" "true"
            fi
        elif command -v wget &>/dev/null; then
            if wget -q -O /tmp/jig_http_$$ --timeout="$timeout_sec" "$url" 2>/dev/null; then
                body=$(cat /tmp/jig_http_$$ 2>/dev/null | head -c 2048 || echo "")
                rm -f /tmp/jig_http_$$
                send_response "HEALTHY: $url is responding.\n\nResponse (truncated):\n$body"
            else
                rm -f /tmp/jig_http_$$
                send_response "UNHEALTHY: $url is not responding." "true"
            fi
        else
            send_response "Error: Neither curl nor wget found. Cannot perform HTTP check." "true"
        fi
        ;;
    *)
        send_response "Unknown check type: '$check'. Valid types: ping, port, http" "true"
        ;;
esac
