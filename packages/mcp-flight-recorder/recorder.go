package flightrecorder

import (
	"context"
	"errors"
)

// mmrFlightRecorder is a FlightRecorder implementation backed by a Merkle
// Mountain Range (MMR). It is parameterised by an MMRHasher and a Store so
// that hashing algorithms and persistence backends can be swapped without
// modifying the core MMR logic.
type mmrFlightRecorder struct {
	hasher MMRHasher
	store  Store
}

// NewFlightRecorder constructs a new MMR-backed FlightRecorder. The storage
// dependency is injected via the Store interface to preserve package
// boundaries and avoid coupling to a specific database.
//
// NOTE: The recorder itself is stateless with respect to MMR index and peaks.
// All state is fetched from and written back to the Store on each Append so
// that the Go monolith remains strictly stateless behind BGP Anycast. The
// Store implementation (e.g. LibSQL single-writer) is responsible for
// serialization and transactional safety.
func NewFlightRecorder(hasher MMRHasher, store Store) FlightRecorder {
	if hasher == nil {
		hasher = NewSHA256Hasher()
	}
	return &mmrFlightRecorder{
		hasher: hasher,
		store:  store,
	}
}

// Append implements the Merkle Mountain Range append algorithm:
//
//  1. Hash the incoming AuditEvent to obtain a leaf hash.
//  2. Construct an MMRLeaf with a monotonic Index and EventID.
//  3. Merge with existing peaks of the same height, hashing parent nodes
//     until the new peak is strictly smaller than the preceding peak.
//  4. Persist the new leaf, internal nodes, updated peaks, and next index
//     via Store.
//
// The cryptographic heart of the algorithm is the peak-merging loop:
// peaks with matching heights are repeatedly combined into parents until
// only one peak of each height remains.
func (r *mmrFlightRecorder) Append(ctx context.Context, event AuditEvent) (MMRLeaf, error) {
	if r.store == nil {
		return MMRLeaf{}, errors.New("flightrecorder: store is nil")
	}

	var leaf MMRLeaf

	// Execute the full Read–Modify–Write sequence inside a single ACID
	// transaction provided by the Store. This prevents interleaving of
	// concurrent appends and keeps the Go layer stateless.
	if err := r.store.RunInTx(ctx, func(ctx context.Context, tx StoreTx) error {
		// Fetch current state from the Store so the recorder itself remains
		// stateless across requests.
		nextIndex, err := tx.GetNextIndex(ctx)
		if err != nil {
			return err
		}

		peaks, err := tx.GetPeaks(ctx)
		if err != nil {
			return err
		}

		leafHash, err := r.hasher.HashLeaf(event)
		if err != nil {
			return err
		}

		localLeaf := MMRLeaf{
			ID:      event.ID,
			Index:   nextIndex,
			EventID: event.ID,
			Hash:    leafHash,
		}
		nextIndex++

		// Start with the new leaf as a height-0 peak.
		currentHash := localLeaf.Hash
		currentHeight := uint64(0)

		// Peak-merging loop:
		// While there exists a previous peak with the same height, merge the
		// two peaks into their parent node and increase the height. This
		// produces a canonical MMR forest where at most one tree exists per
		// height.
		for len(peaks) > 0 && peaks[len(peaks)-1].Height == currentHeight {
			// Pop the last peak.
			prev := peaks[len(peaks)-1]
			peaks = peaks[:len(peaks)-1]

			// Parent = H(prev.Hash || currentHash).
			parentHash, err := r.hasher.HashNode(prev.Hash, currentHash)
			if err != nil {
				return err
			}

			// Persist the internal node so that inclusion proofs can be
			// constructed later.
			if err := tx.SaveNode(ctx, parentHash, prev.Hash, currentHash); err != nil {
				return err
			}

			currentHash = parentHash
			currentHeight++
		}

		// Append the resulting peak.
		peaks = append(peaks, Peak{
			Hash:   currentHash,
			Height: currentHeight,
		})

		// Persist the leaf, peaks, and next index.
		if err := tx.SaveLeaf(ctx, localLeaf); err != nil {
			return err
		}

		if err := tx.SavePeaks(ctx, peaks); err != nil {
			return err
		}

		if err := tx.SaveNextIndex(ctx, nextIndex); err != nil {
			return err
		}

		// Expose the leaf to the caller once the transaction succeeds.
		leaf = localLeaf
		return nil
	}); err != nil {
		return MMRLeaf{}, err
	}

	// Root Sealing hook:
	// A production implementation should asynchronously publish the current
	// MMR root (derived from persisted peaks and size metadata) to an
	// external, immutable public anchor (e.g. transparency log or
	// confidential blockchain) every N events (e.g. 10,000) or on a time
	// schedule.
	//
	// TODO: integrate root sealing trigger when the logical leaf count
	// (tracked in storage) reaches a configurable checkpoint interval.

	return leaf, nil
}

var errProofNotImplemented = errors.New("flightrecorder: inclusion proofs not implemented yet")

// GenerateProof is currently a stub. Implementing full inclusion proofs
// requires storing and traversing the internal MMR nodes, which is beyond
// the scope of this phase.
func (r *mmrFlightRecorder) GenerateProof(leafID string) (MMRInclusionProof, error) {
	return MMRInclusionProof{}, errProofNotImplemented
}
