package flightrecorder

import "context"

// FlightRecorder is the high-level interface for appending audit events into
// an MMR-backed append-only log and generating inclusion proofs.
//
// Trace continuity:
// The Append method accepts a context so that callers can propagate the
// OpenTelemetry traceparent (or equivalent tracing metadata) alongside the
// event. Implementations should extract any required trace identifiers from
// the context and mirror them into their own observability pipeline.
type FlightRecorder interface {
	// Append records a single AuditEvent and returns the corresponding MMR leaf.
	// The provided context must carry any upstream tracing information; this
	// method MUST NOT derive tracing identifiers from global state.
	Append(ctx context.Context, event AuditEvent) (MMRLeaf, error)

	// GenerateProof produces an inclusion proof for the leaf identified by
	// leafID. The exact semantics of leafID (e.g. UUID vs. numeric index)
	// are left to the concrete implementation, but it MUST be stable and
	// unique within a given MMR instance.
	GenerateProof(leafID string) (MMRInclusionProof, error)
}

// MMRHasher defines the pluggable hashing interface used by the underlying
// Merkle Mountain Range implementation. This allows the flight recorder to
// swap between different hash functions or domain separation schemes without
// changing the higher-level APIs.
type MMRHasher interface {
	// HashLeaf computes the hash for a single AuditEvent leaf.
	HashLeaf(event AuditEvent) ([]byte, error)

	// HashNode computes the parent hash from two child node hashes.
	HashNode(left, right []byte) ([]byte, error)
}

