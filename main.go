package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Metric holds the results for a single successful request.
type Metric struct {
	timeToFirstByte time.Duration
	timeToLastByte  time.Duration // This is the total request time
	statusCode      int
}

func main() {
	// --- Step 1, 2, 3, 6: Flag Parsing ---
	urlFlag := flag.String("u", "", "URL to test")
	numReqsFlag := flag.Int("n", 0, "Number of requests")
	concurrencyFlag := flag.Int("c", 0, "Number of concurrent requests")
	fileFlag := flag.String("f", "", "File containing URLs to test")
	flag.Parse()

	// --- Input Validation and URL Loading ---
	urls, err := getURLs(*fileFlag, *urlFlag, flag.Args())
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	numRequests := *numReqsFlag
	concurrency := *concurrencyFlag

	// --- Logic for Step 1 ---
	// If n and c are not set, and we got a bare URL, run as Step 1 (n=1, c=1)
	isStep1Case := *numReqsFlag == 0 && *concurrencyFlag == 0 && len(flag.Args()) > 0
	if isStep1Case {
		numRequests = 1
		concurrency = 1
	}

	if numRequests <= 0 {
		log.Fatal("Error: Number of requests (-n) must be greater than 0")
	}
	if concurrency <= 0 {
		log.Fatal("Error: Concurrency (-c) must be greater than 0")
	}

	// Sanity check: don't start more workers than jobs
	if concurrency > numRequests {
		concurrency = numRequests
	}

	// --- Logic for Step 1 & 2 (Simple Report) vs. Step 3+ (Summary Report) ---
	// The challenge implies simple requests print codes, and load tests print summaries.
	if numRequests <= 10 && concurrency == 1 {
		fmt.Println("Running sequential test...")
		runSequential(urls, numRequests)
	} else {
		fmt.Println("Starting load test...")
		runLoadTest(urls, numRequests, concurrency)
	}
}

// runSequential fulfills Steps 1 & 2, printing individual response codes.
func runSequential(urls []string, n int) {
	client := &http.Client{Timeout: 10 * time.Second}
	for i := range n {
		url := urls[i%len(urls)] // Cycle through URLs
		resp, err := client.Get(url)
		if err != nil {
			fmt.Printf("Request error: %v\n", err)
			continue
		}
		fmt.Printf("Response code: %d\n", resp.StatusCode)
		resp.Body.Close()
	}
}

// runLoadTest fulfills Steps 3-6, running a concurrent test and printing a summary.
func runLoadTest(urls []string, n, c int) {
	jobs := make(chan string, n)
	results := make(chan *Metric, n) // Use *Metric to easily signal network errors with 'nil'

	var wg sync.WaitGroup

	// Create a reusable HTTP client optimized for concurrency
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        c,
			MaxIdleConnsPerHost: c,
		},
		Timeout: 15 * time.Second,
	}

	// --- Step 3: Start Workers ---
	for range c {
		wg.Add(1)
		go worker(&wg, client, jobs, results)
	}

	testStart := time.Now()

	// --- Feed Jobs ---
	for i := range n {
		jobs <- urls[i%len(urls)] // This handles Step 6's "repeat URLs"
	}
	close(jobs) // Signal to workers that no more jobs are coming

	// --- Wait and Close Results ---
	wg.Wait()
	close(results) // Signal to collector that no more results are coming

	testDuration := time.Since(testStart)

	// --- Step 4 & 5: Collect and Analyze Stats ---
	successCount := 0
	failureCount := 0
	ttfbDurations := []time.Duration{}
	ttlbDurations := []time.Duration{}

	for m := range results {
		if m == nil {
			// 'nil' signifies a network-level error (before we got a response)
			failureCount++
		} else {
			// We got a response, so we have metrics
			ttfbDurations = append(ttfbDurations, m.timeToFirstByte)
			ttlbDurations = append(ttlbDurations, m.timeToLastByte)

			if m.statusCode >= 200 && m.statusCode < 300 {
				successCount++
			} else {
				failureCount++
			}
		}
	}

	// --- Calculate final stats ---
	minTTFB, maxTTFB, meanTTFB := analyzeDurations(ttfbDurations)
	minTTLB, maxTTLB, meanTTLB := analyzeDurations(ttlbDurations)
	reqPerSec := float64(n) / testDuration.Seconds()

	// --- Print Report ---
	fmt.Println("\nResults:")
	fmt.Printf(" Total Requests (2XX)..........................: %d\n", successCount)
	fmt.Printf(" Failed Requests (non-2XX or network error)....: %d\n", failureCount)
	fmt.Printf(" Total Requests Per Second.....................: %.2f\n", reqPerSec)
	fmt.Printf("Total Request Time (s) (Min, Max, Mean).......: %.2f, %.2f, %.2f ms\n", minTTLB, maxTTLB, meanTTLB)
	fmt.Printf("Time to First Byte (s) (Min, Max, Mean).......: %.2f, %.2f, %.2f ms\n", minTTFB, maxTTFB, meanTTFB)
	fmt.Printf("Time to Last Byte (s) (Min, Max, Mean)........: %.2f, %.2f, %.2f ms\n", minTTLB, maxTTLB, meanTTLB)
}

// worker is the goroutine that performs the HTTP requests.
// It receives URLs from 'jobs' and sends Metrics (or nil) to 'results'.
func worker(wg *sync.WaitGroup, client *http.Client, jobs <-chan string, results chan<- *Metric) {
	defer wg.Done()
	for url := range jobs {
		start := time.Now()
		resp, err := client.Get(url)
		if err != nil {
			// Network error (e.g., connection refused, DNS lookup failed)
			results <- nil // Send nil to signal failure
			continue
		}
		ttfb := time.Since(start) // This is the true Time to First Byte

		// Ensure the body is read and closed to reuse the connection
		// This is critical for accurate load testing.
		_, err = io.ReadAll(resp.Body)
		resp.Body.Close()

		ttlb := time.Since(start) // This is the true Time to Last Byte (Total Time)

		if err != nil {
			// Body read error
			results <- nil // Count as failure
			continue
		}

		results <- &Metric{
			timeToFirstByte: ttfb,
			timeToLastByte:  ttlb,
			statusCode:      resp.StatusCode,
		}
	}
}

// getURLs figures out the list of URLs to test based on flags.
func getURLs(fileFlag, urlFlag string, args []string) ([]string, error) {
	if fileFlag != "" {
		return readLines(fileFlag)
	}
	if urlFlag != "" {
		return []string{urlFlag}, nil
	}
	if len(args) > 0 {
		return []string{args[0]}, nil
	}
	return nil, fmt.Errorf("no URL provided. Use -u, -f, or a command-line argument")
}

// readLines (for -f flag) reads a file line by line into a string slice.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// analyzeDurations calculates Min, Max, and Mean for a slice of durations.
// Returns all values in milliseconds (ms).
func analyzeDurations(durations []time.Duration) (minMs, maxMs, meanMs float64) {
	if len(durations) == 0 {
		return 0, 0, 0
	}

	minVal := durations[0]
	maxVal := durations[0]
	var totalVal time.Duration

	for _, d := range durations {
		if d < minVal {
			minVal = d
		}
		if d > maxVal {
			maxVal = d
		}
		totalVal += d
	}

	// Convert to ms for reporting
	// Use .Microseconds() for float64 precision
	minMs = float64(minVal.Microseconds()) / 1000.0
	maxMs = float64(maxVal.Microseconds()) / 1000.0
	meanMs = (float64(totalVal.Microseconds()) / 1000.0) / float64(len(durations))

	return
}
