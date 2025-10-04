package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
)

func main() {
	n := flag.Int("n", 0, "number value")

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: ccload <url>")
		os.Exit(2)
	}

	var wg sync.WaitGroup
	url := args[0]

	print("Starting load test...\n")

	for i := 0; i < *n; i++ {
		wg.Go(func() {
			resp, err := http.Get(url)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Request error: %v\n", err)
				return
			}
			defer resp.Body.Close()
			fmt.Printf("Response code: %d\n", resp.StatusCode)
		})
	}
	wg.Wait()
}
