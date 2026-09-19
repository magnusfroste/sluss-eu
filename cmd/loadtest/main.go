// Command loadtest is a small concurrent HTTP load generator for the router's
// /v1/chat/completions endpoint. It measures client-observed end-to-end latency
// under concurrency and reports p50/p95/p99 plus error rate, exiting non-zero if
// p95 exceeds the budget or any request fails.
//
// It is intentionally a dedicated tool, not a bash+curl loop: curl process
// startup adds tens of milliseconds of noise that would swamp a sub-millisecond
// router. Pair it with mock-provider for a deterministic, credential-free
// measurement of routing overhead under load (the beta-gate latency check); the
// router's own internal overhead is also exported at /metrics
// (router_routing_overhead_ms).
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	var (
		url         = flag.String("url", "http://localhost:8080", "router base URL")
		key         = flag.String("key", envOr("LOCAL_API_KEY", "local_router_key"), "API key")
		model       = flag.String("model", "auto", "model to request")
		concurrency = flag.Int("concurrency", 20, "number of concurrent workers")
		requests    = flag.Int("requests", 200, "total number of requests")
		prompt      = flag.String("prompt", "Reply with one word: ok", "prompt content")
		p95Budget   = flag.Float64("p95-budget-ms", 200, "fail if end-to-end p95 exceeds this (ms)")
		timeout     = flag.Duration("timeout", 60*time.Second, "per-request timeout")
	)
	flag.Parse()

	if *concurrency < 1 {
		*concurrency = 1
	}
	if *requests < *concurrency {
		*requests = *concurrency
	}

	body := []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":%q}],"stream":false}`, *model, *prompt))
	client := &http.Client{Timeout: *timeout}

	results := make([]float64, *requests) // ms; -1 marks a failure
	var failures atomic.Int64
	work := make(chan int)
	var wg sync.WaitGroup

	start := time.Now()
	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				ms, err := doRequest(client, *url, *key, body)
				if err != nil {
					results[i] = -1
					failures.Add(1)
					continue
				}
				results[i] = ms
			}
		}()
	}
	for i := 0; i < *requests; i++ {
		work <- i
	}
	close(work)
	wg.Wait()
	elapsed := time.Since(start)

	latencies := make([]float64, 0, *requests)
	for _, r := range results {
		if r >= 0 {
			latencies = append(latencies, r)
		}
	}
	sort.Float64s(latencies)

	failed := failures.Load()
	ok := int64(*requests) - failed
	throughput := float64(*requests) / elapsed.Seconds()

	fmt.Printf("loadtest: %d requests, %d concurrent, %.1fs, %.0f req/s\n", *requests, *concurrency, elapsed.Seconds(), throughput)
	fmt.Printf("  success=%d  failed=%d  error_rate=%.2f%%\n", ok, failed, 100*float64(failed)/float64(*requests))
	if len(latencies) > 0 {
		fmt.Printf("  end-to-end latency (ms): p50=%.1f  p95=%.1f  p99=%.1f  max=%.1f\n",
			percentile(latencies, 0.50), percentile(latencies, 0.95), percentile(latencies, 0.99), latencies[len(latencies)-1])
	}

	// Exit non-zero on any failure or p95 over budget — usable as a CI gate.
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "LOADTEST FAIL: %d request(s) failed\n", failed)
		os.Exit(1)
	}
	if p95 := percentile(latencies, 0.95); p95 > *p95Budget {
		fmt.Fprintf(os.Stderr, "LOADTEST FAIL: p95 %.1fms exceeds budget %.1fms\n", p95, *p95Budget)
		os.Exit(1)
	}
	fmt.Println("LOADTEST PASS")
}

func doRequest(client *http.Client, baseURL, key string, body []byte) (float64, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	t := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	ms := float64(time.Since(t).Microseconds()) / 1000.0
	if resp.StatusCode != http.StatusOK {
		return ms, fmt.Errorf("status %d", resp.StatusCode)
	}
	return ms, nil
}

// percentile returns the p-quantile (0..1) of an ascending-sorted slice using
// the nearest-rank method. Returns 0 for an empty slice.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[n-1]
	}
	rank := int(p*float64(n) + 0.999999) // ceil
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
