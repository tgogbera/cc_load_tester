package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
)

func main() {
	// Use the flag package only to allow future extensions; parse any flags first
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: ccload <url>")
		os.Exit(2)
	}

	url := args[0]

	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	fmt.Printf("Response code: %d\n", resp.StatusCode)
}
