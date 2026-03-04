package flightrecorder

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// appendRequest is sent by Append and processed by the batch worker.
type appendRequest struct {
	event   AuditEvent
	resultCh chan appendResult
}

// appendResult is sent back to the caller on resultCh.
type appendResult struct {
	leaf MMRLeaf
	err  error
}

// mmrFlightRecorder is a FlightRecorder implementation backed by a Merkle
// Mountain Range (MMR). It uses a background batch worker to aggregate
// appends and execute a single RunInTx per batch (up to 500 items or 50ms).
type mmrFlightRecorder struct {
	hasher MMRHasher
	store  Store
	reqCh  chan appendRequest
}

const (
	batchChanBuffer = 1000
	batchSizeMax    = 500
	batchWindow     = 50 * time.Millisecond
)

// NewFlightRecorder constructs a new MMR-backed FlightRecorder. The storage
// dependency is injected via the Store interface. A background batch worker
// is started; handlers enqueue via reqCh and block on resultCh.
func NewFlightRecorder(hasher MMRHasher, store Store) FlightRecorder {
	if hasher == nil {
		hasher = NewSHA256Hasher()
	}
	r := &mmrFlightRecorder{
		hasher: hasher,
		store:  store,
		reqCh:  make(chan appendRequest, batchChanBuffer),
	}
	go r.batchWorker()
	return r
}

// Append enqueues the event and blocks until the batch worker processes it
// or the context is cancelled.
func (r *mmrFlightRecorder) Append(ctx context.Context, event AuditEvent) (MMRLeaf, error) {
	if r.store == nil {
		return MMRLeaf{}, errors.New("flightrecorder: store is nil")
	}

	resultCh := make(chan appendResult, 1)
	req := appendRequest{event: event, resultCh: resultCh}

	select {
	case r.reqCh <- req:
		// Enqueued; wait for result or cancellation.
		select {
		case res := <-resultCh:
			return res.leaf, res.err
		case <-ctx.Done():
			return MMRLeaf{}, ctx.Err()
		}
	case <-ctx.Done():
		return MMRLeaf{}, ctx.Err()
	}
}

// batchWorker aggregates up to batchSizeMax requests or a batchWindow tick,
// then runs a single RunInTx: fetch state once, process all items (peak-merge
// and persist leaves/nodes), then write peaks and next index once.
func (r *mmrFlightRecorder) batchWorker() {
	ticker := time.NewTicker(batchWindow)
	defer ticker.Stop()

	var batch []appendRequest
	flush := func() {
		if len(batch) == 0 {
			return
		}
		r.processBatch(batch)
		batch = nil
	}

	for {
		select {
		case req := <-r.reqCh:
			batch = append(batch, req)
			if len(batch) >= batchSizeMax {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// processBatch runs one RunInTx: GetNextIndex and GetPeaks once, then for
// each request does the peak-merging math (SaveNode, SaveLeaf), updates
// in-memory nextIndex and peaks, then SavePeaks and SaveNextIndex once.
// Replies to each request's resultCh with the leaf or error.
func (r *mmrFlightRecorder) processBatch(batch []appendRequest) {
	ctx := context.Background()
	tracer := otel.Tracer("mcp-flight-recorder")
	ctx, span := tracer.Start(ctx, "mmr.process_batch")
	defer span.End()
	span.SetAttributes(attribute.Int("batch.size", len(batch)))

	results := make([]appendResult, len(batch))

	err := r.store.RunInTx(ctx, func(ctx context.Context, tx StoreTx) error {
		nextIndex, err := tx.GetNextIndex(ctx)
		if err != nil {
			return err
		}
		peaks, err := tx.GetPeaks(ctx)
		if err != nil {
			return err
		}

		for i := range batch {
			event := batch[i].event
			leafHash, err := r.hasher.HashLeaf(event)
			if err != nil {
				results[i] = appendResult{err: err}
				return err
			}
			localLeaf := MMRLeaf{
				ID:           event.ID,
				Index:        nextIndex,
				TenantID:     event.TenantID,
				EventID:      event.ID,
				Hash:         leafHash,
				PQCSignature: event.PQCSignature,
				PQCPublicKey: event.PQCPublicKey,
			}
			nextIndex++

			currentHash := localLeaf.Hash
			currentHeight := uint64(0)

			for len(peaks) > 0 && peaks[len(peaks)-1].Height == currentHeight {
				prev := peaks[len(peaks)-1]
				peaks = peaks[:len(peaks)-1]

				parentHash, err := r.hasher.HashNode(prev.Hash, currentHash)
				if err != nil {
					results[i] = appendResult{err: err}
					return err
				}
				if err := tx.SaveNode(ctx, parentHash, prev.Hash, currentHash); err != nil {
					results[i] = appendResult{err: err}
					return err
				}

				currentHash = parentHash
				currentHeight++
			}

			peaks = append(peaks, Peak{
				Hash:   currentHash,
				Height: currentHeight,
			})

			if err := tx.SaveLeaf(ctx, localLeaf); err != nil {
				results[i] = appendResult{err: err}
				return err
			}

			results[i] = appendResult{leaf: localLeaf}
		}

		if err := tx.SavePeaks(ctx, peaks); err != nil {
			return err
		}
		return tx.SaveNextIndex(ctx, nextIndex)
	})

	if err != nil {
		for i := range results {
			if results[i].err == nil {
				results[i] = appendResult{err: err}
			}
		}
	}

	for i, req := range batch {
		select {
		case req.resultCh <- results[i]:
		default:
			// resultCh is buffered(1); should not block
		}
	}
}

var errProofNotImplemented = errors.New("flightrecorder: inclusion proofs not implemented yet")

// GenerateProof is currently a stub.
func (r *mmrFlightRecorder) GenerateProof(leafID string) (MMRInclusionProof, error) {
	return MMRInclusionProof{}, errProofNotImplemented
}
