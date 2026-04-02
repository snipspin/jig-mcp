//go:build ignore

// sleep_tool is a test binary that sleeps for a configurable duration.
// Used by integration tests to verify execution timeout (TASK-04).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func main() {
	dur := 60 * time.Second // default: sleep longer than any reasonable timeout

	if len(os.Args) >= 2 {
		raw := os.Args[len(os.Args)-1]
		var args map[string]any
		if err := json.Unmarshal([]byte(raw), &args); err == nil {
			if s, ok := args["duration"].(string); ok {
				if d, err := time.ParseDuration(s); err == nil {
					dur = d
				}
			}
		}
	}

	time.Sleep(dur)

	resp := map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": fmt.Sprintf("slept for %s", dur),
		}},
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
}
