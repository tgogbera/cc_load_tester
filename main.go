package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

type Metrics struct {
	requestTime     float64
	timeToFirstByte float64
	timeToLastByte  float64
}

func main() {
	n := flag.Int("n", 0, "number value")
	u := flag.String("u", "", "url value")
	c := flag.Int("c", 0, "concurent requests value")

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

	if *c <= 0 {
		fmt.Fprintln(os.Stderr, "Number of concurrent requests must be greater than 0")
		os.Exit(1)
	}

	successCount := 0
	failureCount := 0

	seqReqs := *n - *c

	metrics := []Metrics{}

	var wg sync.WaitGroup

	for range seqReqs {
		start := time.Now()
		resp, err := http.Get(url)
		end := time.Now()
		totalTime := end.Sub(start).Seconds() * 1000
		m := Metrics{requestTime: totalTime}
		if err != nil {
			fmt.Println("Request error:", err)
			return
		}
		firstByteTime := time.Now()
		defer resp.Body.Close()

		io.ReadAll(resp.Body)
		lastByteTime := time.Now()

		ttfb := firstByteTime.Sub(start).Seconds() * 1000
		ttlb := lastByteTime.Sub(start).Seconds() * 1000

		m.timeToFirstByte = ttfb
		m.timeToLastByte = ttlb

		metrics = append(metrics, m)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			successCount++
		} else {
			failureCount++
		}
	}

	print("Starting load test...\n")

	for i := 0; i < *c; i++ {
		wg.Go(func() {
			start := time.Now()
			resp, err := http.Get(url)
			end := time.Now()
			totalTime := end.Sub(start).Seconds() * 1000
			m := Metrics{requestTime: totalTime}
			if err != nil {
				fmt.Println("Request error:", err)
				return
			}
			firstByteTime := time.Now()
			defer resp.Body.Close()

			io.ReadAll(resp.Body)
			lastByteTime := time.Now()

			ttfb := firstByteTime.Sub(start).Seconds() * 1000
			ttlb := lastByteTime.Sub(start).Seconds() * 1000

			m.timeToFirstByte = ttfb
			m.timeToLastByte = ttlb

			metrics = append(metrics, m)

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				successCount++
			} else {
				failureCount++
			}
		})
	}
	wg.Wait()

	minReqTime := _minReqTime(metrics)
	maxReqTime := _maxReqTime(metrics)
	meanReqTime := _meanReqTime(metrics)
	minFirstByteTime := _minFirstByteTime(metrics)
	maxFirstByteTime := _maxFirstByteTime(metrics)
	meanFirstByteTime := _meanFirstByteTime(metrics)
	minLastByteTime := _minLastByteTime(metrics)
	maxLastByteTime := _maxLastByteTime(metrics)
	meanLastByteTime := _meanLastByteTime(metrics)

	fmt.Printf("Total Requests (2XX)..........................: %d\n", successCount)
	fmt.Printf("Failed Requests (5XX).........................: %d\n", failureCount)
	fmt.Printf("Total Requests Per Second.....................: %.2f\n", _totalRequestsPerSecond(metrics))
	fmt.Printf("Total Request Time (s) (Min, Max, Mean).......: %.2f, %.2f, %.2f ms\n", minReqTime, maxReqTime, meanReqTime)
	fmt.Printf("Time to First Byte (s) (Min, Max, Mean).......: %.2f, %.2f, %.2f ms\n", minFirstByteTime, maxFirstByteTime, meanFirstByteTime)
	fmt.Printf("Time to Last Byte (s) (Min, Max, Mean)........: %.2f , %.2f, %.2f ms\n", minLastByteTime, maxLastByteTime, meanLastByteTime)

}

func _totalRequestsPerSecond(metrics []Metrics) float64 {
	totalTime := 0.0

	for _, r := range metrics {
		totalTime += r.requestTime
	}

	reqPerSec := float64(len(metrics)) / (totalTime / 1000)
	return reqPerSec
}

func _minReqTime(metrics []Metrics) float64 {
	var minReqTime float64

	for i, r := range metrics {
		if i == 0 || r.requestTime < minReqTime {
			minReqTime = r.requestTime
		}
	}
	return minReqTime
}

func _maxReqTime(metrics []Metrics) float64 {
	var maxReqTime float64

	for i, r := range metrics {
		if i == 0 || r.requestTime > maxReqTime {
			maxReqTime = r.requestTime
		}
	}
	return maxReqTime
}

func _meanReqTime(metrics []Metrics) float64 {
	var totalReqTime float64

	for _, r := range metrics {
		totalReqTime += r.requestTime
	}
	meanReqTime := totalReqTime / float64(len(metrics))
	return meanReqTime
}

func _minFirstByteTime(metrics []Metrics) float64 {
	var minFirstByteTime float64

	for i, r := range metrics {
		if i == 0 || r.timeToFirstByte < minFirstByteTime {
			minFirstByteTime = r.timeToFirstByte
		}
	}
	return minFirstByteTime
}

func _maxFirstByteTime(metrics []Metrics) float64 {
	var maxFirstByteTime float64

	for i, r := range metrics {
		if i == 0 || r.timeToFirstByte > maxFirstByteTime {
			maxFirstByteTime = r.timeToFirstByte
		}
	}
	return maxFirstByteTime
}

func _meanFirstByteTime(metrics []Metrics) float64 {
	var totalFirstByteTime float64

	for _, r := range metrics {
		totalFirstByteTime += r.timeToFirstByte
	}
	meanFirstByteTime := totalFirstByteTime / float64(len(metrics))
	return meanFirstByteTime
}

func _minLastByteTime(metrics []Metrics) float64 {
	var minLastByteTime float64

	for i, r := range metrics {
		if i == 0 || r.timeToLastByte < minLastByteTime {
			minLastByteTime = r.timeToLastByte
		}
	}
	return minLastByteTime
}

func _maxLastByteTime(metrics []Metrics) float64 {
	var maxLastByteTime float64
	for i, r := range metrics {
		if i == 0 || r.timeToLastByte > maxLastByteTime {
			maxLastByteTime = r.timeToLastByte
		}
	}
	return maxLastByteTime
}

func _meanLastByteTime(metrics []Metrics) float64 {
	var totalLastByteTime float64

	for _, r := range metrics {
		totalLastByteTime += r.timeToLastByte
	}
	meanLastByteTime := totalLastByteTime / float64(len(metrics))
	return meanLastByteTime
}
