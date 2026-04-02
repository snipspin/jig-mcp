#!/bin/bash
JSON_ARGS=$1

# --- Configuration ---
ALLOWED_BASE_DIRS=(
    "$(pwd)"
    "$HOME"
    "/tmp"
)

# --- Utilities ---
send_response() {
    local result=$1
    local is_error=$2
    local response
    if [ "$is_error" = "true" ]; then
        response=$(printf '{"content":[{"type":"text","text":"%s"}],"isError":true}' "$result")
    else
        # If result is already JSON (like an array from list_dir), use it directly.
        # Otherwise, wrap as string. Simple check: if starts with [ or {
        if [[ $result == [* || $result == {* ]]; then
            response=$(printf '{"content":[{"type":"text","text":%s}]}' "$result")
        else
            # Pre-escape double quotes for string content.
            escaped_result=$(echo "$result" | sed 's/"/\\"/g' | tr -d '\n\r')
            response=$(printf '{"content":[{"type":"text","text":"%s"}]}' "$escaped_result")
        fi
    fi
    echo "$response"
    exit $([ "$is_error" = "true" ] && echo 1 || echo 0)
}

validate_path() {
    local path=$1
    if [ -z "$path" ]; then
        echo "Path argument is required." >&2
        return 1
    fi

    # Simple absolute path conversion (assuming relative to PWD)
    if [[ "$path" != /* ]]; then path="$(pwd)/$path"; fi

    # Canonicalize to resolve .. and symlinks, preventing path traversal.
    # Use realpath if available (GNU coreutils), fall back to readlink -f.
    if command -v realpath &>/dev/null; then
        path=$(realpath -m "$path" 2>/dev/null) || true
    elif command -v readlink &>/dev/null; then
        path=$(readlink -f "$path" 2>/dev/null) || true
    fi

    local allowed=false
    for base in "${ALLOWED_BASE_DIRS[@]}"; do
        if [[ "$path" == "$base" || "$path" == "$base/"* ]]; then
            allowed=true
            break
        fi
    done

    if [ "$allowed" = "false" ]; then
        echo "Access denied: Path '$path' is not within allowed base directories." >&2
        return 1
    fi
    echo "$path"
    return 0
}

# --- Main (Simple JSON parsing via jq if available, else manual extraction for basics) ---
# Since jq might not be in the host, and this is a script that must run on host:
# For now, let's assume jq is NOT available and use simple string matching for 'operation' etc.
operation=$(echo "$JSON_ARGS" | sed -n 's/.*"operation"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
path_arg=$(echo "$JSON_ARGS" | sed -n 's/.*"path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
root_arg=$(echo "$JSON_ARGS" | sed -n 's/.*"root"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
pattern_arg=$(echo "$JSON_ARGS" | sed -n 's/.*"pattern"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
content_arg=$(echo "$JSON_ARGS" | sed -n 's/.*"content"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
lines_arg=$(echo "$JSON_ARGS" | sed -n 's/.*"lines"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')

case "$operation" in
    "read_file")
        path=$(validate_path "$path_arg" 2>&1) || send_response "$path" "true"
        if [ ! -f "$path" ]; then send_response "File not found: $path" "true"; fi
        content=$(cat "$path") || send_response "Error reading file: $path" "true"
        send_response "$content"
        ;;
    "write_file")
        path=$(validate_path "$path_arg" 2>&1) || send_response "$path" "true"
        mkdir -p "$(dirname "$path")"
        echo -n "$content_arg" > "$path" || send_response "Error writing file: $path" "true"
        send_response "Successfully wrote to $path"
        ;;
    "list_dir")
        path=$(validate_path "$path_arg" 2>&1) || send_response "$path" "true"
        if [ ! -d "$path" ]; then send_response "Directory not found: $path" "true"; fi
        # Simple JSON array construction using ls
        items="["
        first=true
        for entry in "$path"/*; do
            [ -e "$entry" ] || continue
            if [ "$first" = false ]; then items+=","; fi
            name=$(basename "$entry")
            isDir=$([ -d "$entry" ] && echo true || echo false)
            size=$([ -f "$entry" ] && stat -c%s "$entry" || echo null)
            items+="{\"name\":\"$name\",\"isDir\":$isDir,\"size\":$size}"
            first=false
        done
        items+="]"
        send_response "$items"
        ;;
    "search_files")
        root=$(validate_path "$root_arg" 2>&1) || send_response "$root" "true"
        [ -z "$pattern_arg" ] && send_response "Pattern is required for search_files." "true"
        results=$(find "$root" -name "$pattern_arg") || send_response "Error searching files in $root" "true"
        # Convert results to JSON array
        json_results=$(printf '%s\n' "$results" | sed 's/"/\\"/g' | awk 'BEGIN{printf "["} {if(NR>1)printf ","; printf "\"%s\"",$0} END{printf "]"}' | tr -d '\n\r')
        send_response "$json_results"
        ;;
    "read_log")
        path=$(validate_path "$path_arg" 2>&1) || send_response "$path" "true"
        lines=${lines_arg:-10}
        if [ ! -f "$path" ]; then send_response "Log file not found: $path" "true"; fi
        content=$(tail -n "$lines" "$path") || send_response "Error reading log: $path" "true"
        send_response "$content"
        ;;
    *)
        send_response "Unknown operation: $operation" "true"
        ;;
esac
