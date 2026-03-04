package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"time"
)

// WebhookEvent is a single event to be delivered to an endpoint (e.g. FinOps settlement).
type WebhookEvent struct {
	EndpointURL  string
	Payload      []byte
	TenantSecret string
}

// Dispatcher runs an asynchronous worker that delivers events with HMAC-SHA256 signing and exponential backoff.
type Dispatcher struct {
	queue chan WebhookEvent
	client *http.Client
}

// NewDispatcher creates a Dispatcher with the given queue size and starts the background worker.
// Queue size should be large enough to avoid blocking enqueuers (e.g. 1000).
func NewDispatcher(queueSize int) *Dispatcher {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	d := &Dispatcher{
		queue:  make(chan WebhookEvent, queueSize),
		client: client,
	}
	go d.worker()
	return d
}

// Enqueue sends the event to the worker. Non-blocking if the queue has capacity; otherwise drops or blocks depending on channel semantics.
// Using a buffered channel: enqueue blocks when full. We use a reasonable buffer so FinOps settlement doesn't block long.
func (d *Dispatcher) Enqueue(ev WebhookEvent) {
	select {
	case d.queue <- ev:
	default:
		log.Printf("webhooks: queue full, dropping event for %s", ev.EndpointURL)
	}
}

// worker processes the queue: for each event, sign payload with HMAC-SHA256 and POST with retries (1s, 2s, 4s).
func (d *Dispatcher) worker() {
	for ev := range d.queue {
		d.deliver(ev)
	}
}

func (d *Dispatcher) deliver(ev WebhookEvent) {
	signature := signPayload(ev.Payload, ev.TenantSecret)
	// Retry up to 3 times: initial try, then wait 1s, 2s, 4s before next try.
	backoffs := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(backoffs[attempt-1])
		}
		req, err := http.NewRequest(http.MethodPost, ev.EndpointURL, bytes.NewReader(ev.Payload))
		if err != nil {
			log.Printf("webhooks: invalid request for %s: %v", ev.EndpointURL, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Vericore-Signature", "sha256="+signature)

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < 2 {
				continue
			}
			log.Printf("webhooks: delivery failed after 3 attempts to %s: %v", ev.EndpointURL, err)
			return
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			lastErr = nil
			if attempt < 2 {
				continue
			}
			log.Printf("webhooks: delivery failed after 3 attempts to %s: status %d", ev.EndpointURL, resp.StatusCode)
			return
		}
		// 4xx (other than 429): do not retry
		log.Printf("webhooks: endpoint %s returned %d, not retrying", ev.EndpointURL, resp.StatusCode)
		return
	}
	if lastErr != nil {
		log.Printf("webhooks: delivery failed to %s: %v", ev.EndpointURL, lastErr)
	}
}

// signPayload returns HMAC-SHA256 of payload using secret, hex-encoded.
func signPayload(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// Stop closes the queue channel so the worker exits. Call when shutting down the server.
func (d *Dispatcher) Stop() {
	close(d.queue)
}
