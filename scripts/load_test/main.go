// Chaos load test: prove the single-writer MMR and LibSQL pool survive
// 5,000 concurrent POSTs without SQLITE_BUSY or data corruption.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	baseURL    = "http://localhost:8080"
	concurrency = 5000
	// Valid payload: passes guardrail, triggers MMR append + DB write.
	dummyFIDO = "ZHVtbXlfZmlkb19zaWduYXR1cmU="
)

func main() {
	fmt.Println("=== V3R1C0R3 Chaos Load Test ===")
	fmt.Printf("Firing %d concurrent POSTs to %s/api/v1/agent/action\n", concurrency, baseURL)
	fmt.Println()

	var ok200, err403, err500 int64
	var busyOrLocked int64
	var errorSamples []string
	var errorMu sync.Mutex
	const maxSamples = 20

	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			body := mustJSON(map[string]interface{}{
				"action_id":       fmt.Sprintf("load-test-%d", n),
				"decision":        "APPROVED",
				"reasoning":       "chaos load test",
				"fido_signature":  dummyFIDO,
				"record_id":       "1",
				"expected_state":  "committed",
			})
			resp, err := http.Post(baseURL+"/api/v1/agent/action", "application/json", bytes.NewReader(body))
			if err != nil {
				atomic.AddInt64(&err500, 1)
				errorMu.Lock()
				if len(errorSamples) < maxSamples {
					errorSamples = append(errorSamples, "client: "+err.Error())
				}
				errorMu.Unlock()
				return
			}
			defer resp.Body.Close()
			slurp, _ := io.ReadAll(resp.Body)
			bodyStr := strings.TrimSpace(string(slurp))
			lower := strings.ToLower(bodyStr)

			switch resp.StatusCode {
			case http.StatusOK:
				atomic.AddInt64(&ok200, 1)
			case http.StatusForbidden:
				atomic.AddInt64(&err403, 1)
				errorMu.Lock()
				if len(errorSamples) < maxSamples {
					errorSamples = append(errorSamples, fmt.Sprintf("403: %s", bodyStr))
				}
				errorMu.Unlock()
			default:
				atomic.AddInt64(&err500, 1)
				errorMu.Lock()
				if len(errorSamples) < maxSamples {
					errorSamples = append(errorSamples, fmt.Sprintf("%d: %s", resp.StatusCode, bodyStr))
				}
				errorMu.Unlock()
			}

			if strings.Contains(lower, "database is locked") || strings.Contains(lower, "busy") || strings.Contains(lower, "sqlite_busy") {
				atomic.AddInt64(&busyOrLocked, 1)
				errorMu.Lock()
				if len(errorSamples) < maxSamples {
					errorSamples = append(errorSamples, "[LOCK/BUSY] "+bodyStr)
				}
				errorMu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// --- Metrics & reporting ---
	total := ok200 + err403 + err500
	rps := float64(total) / elapsed.Seconds()

	fmt.Println("--- Metrics ---")
	fmt.Printf("  HTTP 200 (success):     %d\n", ok200)
	fmt.Printf("  HTTP 403 (forbidden):   %d\n", err403)
	fmt.Printf("  HTTP 500 / client err:  %d\n", err500)
	fmt.Printf("  Total requests:         %d\n", total)
	fmt.Printf("  Duration:               %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Requests per second:    %.2f RPS\n", rps)
	fmt.Printf("  Responses with locked/busy: %d\n", busyOrLocked)
	if len(errorSamples) > 0 {
		fmt.Println("\n--- Error sample (up to 20) ---")
		for _, s := range errorSamples {
			if len(s) > 120 {
				s = s[:117] + "..."
			}
			fmt.Printf("  %s\n", s)
		}
	}

	// --- Post-attack integrity check ---
	fmt.Println("\n--- Post-attack integrity check ---")
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/telemetry/stats", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  FAIL: could not fetch telemetry: %v\n", err)
		return
	}
	defer resp.Body.Close()
	slurp, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  FAIL: telemetry returned %d: %s\n", resp.StatusCode, string(slurp))
		return
	}
	var stats struct {
		TotalEvents int64 `json:"total_events"`
	}
	if err := json.Unmarshal(slurp, &stats); err != nil {
		fmt.Printf("  FAIL: could not parse telemetry JSON: %v\n", err)
		return
	}
	fmt.Printf("  total_events (MMR): %d\n", stats.TotalEvents)
	fmt.Printf("  successful POSTs:  %d\n", ok200)
	if int64(ok200) <= stats.TotalEvents {
		fmt.Println("  Integrity: MMR total_events >= successful POSTs (no drops detected).")
	} else {
		fmt.Println("  WARN: total_events < successful POSTs (possible race or prior state).")
	}
	fmt.Println("\n=== Load test complete ===")
}

func mustJSON(v map[string]interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
