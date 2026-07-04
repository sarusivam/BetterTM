package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("🚀 Dummy server started on port 9000...")
	fmt.Println("Press Ctrl+C to close it manually, or kill it with your app!")

	// Listen on port 9000
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		fmt.Printf("Could not start port: %v\n", err)
	}
}
