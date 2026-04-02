//go:build ignore

// echo_tool is a minimal binary that reads JSON arguments from the command line
// and returns a valid MCP tool response on stdout. Used by integration tests to
// verify binary-only distribution support (TASK-03).
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		resp := map[string]any{
			"content": []map[string]any{{
				"type": "text",
				"text": "no arguments provided",
			}},
			"isError": true,
		}
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
		return
	}

	// Last argument is the JSON-encoded arguments from jig-mcp.
	raw := os.Args[len(os.Args)-1]

	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		resp := map[string]any{
			"content": []map[string]any{{
				"type": "text",
				"text": fmt.Sprintf("invalid JSON: %v", err),
			}},
			"isError": true,
		}
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
		return
	}

	msg, _ := args["message"].(string)
	if msg == "" {
		msg = "echo"
	}

	resp := map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": fmt.Sprintf("binary-echo: %s", msg),
		}},
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
}
