//go:build ignore

// oom_tool is a test binary that allocates memory in a loop until killed.
// Used by integration tests to verify resource limits (TASK-06).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
)

func main() {
	sizeMB := 1024 // default: try to allocate 1 GB

	if len(os.Args) >= 2 {
		raw := os.Args[len(os.Args)-1]
		var args map[string]any
		if err := json.Unmarshal([]byte(raw), &args); err == nil {
			if s, ok := args["sizeMB"].(float64); ok {
				sizeMB = int(s)
			}
		}
	}

	// Allocate in 1 MB chunks, touching every page to ensure commit.
	var chunks [][]byte
	for i := 0; i < sizeMB; i++ {
		chunk := make([]byte, 1024*1024)
		for j := 0; j < len(chunk); j += 4096 {
			chunk[j] = byte(i)
		}
		chunks = append(chunks, chunk)
	}

	// Keep chunks alive so GC doesn't collect them.
	runtime.KeepAlive(chunks)

	resp := map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": fmt.Sprintf("allocated %d MB", sizeMB),
		}},
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
}
