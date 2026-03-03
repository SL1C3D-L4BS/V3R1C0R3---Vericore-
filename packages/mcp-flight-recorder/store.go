package flightrecorder

import "context"

// Peak represents a single MMR peak persisted by the Store. Height MUST be
// stored so that the recorder can reconstruct the MMR forest shape without
// keeping in-memory state across appends (the monolith is strictly stateless).
type Peak struct {
	Hash   []byte
	Height uint64
}

// StoreTx represents the transactional unit-of-work used by the flight
// recorder. All MMR reads and writes for a single Append must be performed
// via a single StoreTx to guarantee atomicity and correct sequencing.
//
// NOTE: Implementations MUST execute StoreTx methods within a single ACID
// transaction (e.g. using BEGIN IMMEDIATE in SQLite/LibSQL). The Go monolith
// itself remains stateless; transactional safety is enforced at the storage
// layer.
type StoreTx interface {
	// SaveLeaf persists a single MMR leaf. Implementations SHOULD treat this
	// as an idempotent upsert keyed by leaf.ID.
	SaveLeaf(ctx context.Context, leaf MMRLeaf) error

	// GetLeaf looks up a previously stored MMR leaf by its ID.
	GetLeaf(ctx context.Context, id string) (MMRLeaf, error)

	// GetPeaks returns the current set of MMR peaks, ordered from left to
	// right (oldest/smallest index to newest/highest index).
	GetPeaks(ctx context.Context) ([]Peak, error)

	// SavePeaks replaces the current set of MMR peaks with the provided
	// slice. Implementations SHOULD perform this update atomically with
	// any related leaf/index/node writes when used in production.
	SavePeaks(ctx context.Context, peaks []Peak) error

	// GetNextIndex returns the next logical leaf index for the MMR.
	GetNextIndex(ctx context.Context) (uint64, error)

	// SaveNextIndex persists the next logical leaf index for the MMR.
	SaveNextIndex(ctx context.Context, next uint64) error

	// SaveNode persists an internal MMR node created during peak merging.
	// The hash identifies the parent; leftHash and rightHash are its
	// children. This data is required later to construct O(log N) inclusion
	// proofs in GenerateProof.
	SaveNode(ctx context.Context, hash []byte, leftHash []byte, rightHash []byte) error
}

// Store is the top-level storage abstraction. It owns the transaction
// boundaries and runs the entire Read–Modify–Write MMR append sequence
// inside a single ACID transaction via RunInTx.
type Store interface {
	// RunInTx executes fn inside a single ACID transaction that provides
	// a StoreTx for all reads/writes. Implementations MUST ensure that:
	//   - The transaction uses BEGIN IMMEDIATE (or equivalent) to avoid
	//     SQLITE_BUSY under high concurrent load.
	//   - fn is either fully committed or fully rolled back.
	RunInTx(ctx context.Context, fn func(ctx context.Context, tx StoreTx) error) error
}



