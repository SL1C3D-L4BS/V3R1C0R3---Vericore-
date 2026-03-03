package flightrecorder

import (
	"crypto/sha256"
	"encoding/json"
)

// sha256Hasher is a simple, domain-neutral implementation of MMRHasher that
// uses SHA-256 for both leaf and internal node hashing.
//
// This is sufficient for initial bring-up and testing. Production deployments
// may choose to swap in a different implementation (e.g. domain-separated
// hashes or a different digest algorithm) without changing the recorder.
type sha256Hasher struct{}

// NewSHA256Hasher returns a default MMRHasher based on SHA-256.
func NewSHA256Hasher() MMRHasher {
	return sha256Hasher{}
}

// HashLeaf computes the hash for a single AuditEvent leaf by JSON-encoding
// the event and hashing the resulting bytes.
func (h sha256Hasher) HashLeaf(event AuditEvent) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	return sum[:], nil
}

// HashNode computes the parent hash from two child node hashes by
// concatenating them in left/right order and hashing the result.
func (h sha256Hasher) HashNode(left, right []byte) ([]byte, error) {
	combined := make([]byte, 0, len(left)+len(right))
	combined = append(combined, left...)
	combined = append(combined, right...)
	sum := sha256.Sum256(combined)
	return sum[:], nil
}

