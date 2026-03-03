package zkp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// AsyncCoprocessor is a stub Prover that simulates enqueueing the payload to a
// durable queue (or Bonsai network) instead of blocking the main Go thread.
// It returns a synthetic receiptRef and the SHA-256 hash of the payload as
// publicHash, without performing a real zkVM proof.
type AsyncCoprocessor struct {
	mu      sync.Mutex
	counter int
	// queue simulates a durable queue; in production this would be a real queue
	// (e.g. Redis, SQS) or a Bonsai client that enqueues proof requests.
	queue []queuedJob
}

type queuedJob struct {
	payload []byte
	ref     string
}

// NewAsyncCoprocessor returns a stub prover that enqueues and returns immediately.
func NewAsyncCoprocessor() *AsyncCoprocessor {
	return &AsyncCoprocessor{}
}

// GenerateReceipt implements Prover. It simulates enqueueing privatePayload to
// a durable queue / Bonsai; it does not block on proof generation. It returns
// a synthetic receiptRef and the SHA-256 hash of the payload as publicHash.
func (a *AsyncCoprocessor) GenerateReceipt(ctx context.Context, privatePayload []byte) (receiptRef string, publicHash []byte, err error) {
	if ctx != nil && ctx.Err() != nil {
		return "", nil, ctx.Err()
	}
	if len(privatePayload) == 0 {
		return "", nil, fmt.Errorf("zkp: private payload is empty")
	}

	h := sha256.Sum256(privatePayload)
	publicHash = h[:]

	a.mu.Lock()
	a.counter++
	ref := fmt.Sprintf("bonsai-sim-%d-%s", a.counter, hex.EncodeToString(publicHash)[:16])
	a.queue = append(a.queue, queuedJob{payload: privatePayload, ref: ref})
	a.mu.Unlock()

	return ref, publicHash, nil
}
