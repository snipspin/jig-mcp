#!/bin/bash
# hello.sh - Example jig-mcp tool script
# Reads JSON arguments from stdin and outputs a greeting

# Read the JSON argument (passed as last argument by jig-mcp)
ARGS="$1"

# Extract the name parameter (default to "World" if not provided)
NAME=$(echo "$ARGS" | jq -r '.name // "World"')

# Output the greeting as a JSON object
# jig-mcp accepts either a raw string or an MCP-compliant response
# Here we use the MCP format for demonstration
jq -n --arg msg "Hello, $NAME! This is an example jig-mcp tool." '{
  content: [{
    type: "text",
    text: $msg
  }]
}'
