package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println(`{"content": [{"type": "text", "text": "error: no arguments provided"}], "isError": true}`)
		return
	}

	// The last argument should be our JSON input, e.g. {"name": "test"}
	arg := os.Args[len(os.Args)-1]

	var params map[string]any
	if err := json.Unmarshal([]byte(arg), &params); err != nil {
		fmt.Printf(`{"content": [{"type": "text", "text": "error: invalid JSON input: %v"}], "isError": true}`, err)
		return
	}

	resp := map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": fmt.Sprintf("Binary tool received arguments: %v", params),
			},
		},
	}

	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
}
