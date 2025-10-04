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
	u := flag.String("u", "", "url value")

	flag.Parse()

	url := *u
	if url == "" {
		fmt.Fprintln(os.Stderr, "URL is required")
		os.Exit(1)
	}

	if *n <= 0 {
		fmt.Fprintln(os.Stderr, "Number of requests must be greater than 0")
		os.Exit(1)
	}

	var wg sync.WaitGroup

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
