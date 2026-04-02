//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println(`{"content": [{"type": "text", "text": "no args"}]}`)
		return
	}

	// Output a JSON array (not a map) to test non-map response handling
	output := []string{"item1", "item2", "item3"}
	data, _ := json.Marshal(output)
	fmt.Println(string(data))
}
