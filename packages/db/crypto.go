package db

import "context"

// EnvelopeManager provides GDPR Right to Erasure via envelope encryption and
// key shredding. Plaintext is encrypted with a Data Encryption Key (DEK);
// the DEK is itself protected by a Key Encryption Key (KEK). Only ciphertext
// and key identifiers cross the boundary into the MMR audit log (e.g.
// ParamsJSON in AuditEvent). Destroying the DEK via ShredKey makes that
// ciphertext permanently unrecoverable while leaving the cryptographic hash
// (EnvelopeHash) intact for inclusion proofs and non-repudiation.
type EnvelopeManager interface {
	// Encrypt encrypts plaintext for the given tenant and returns the
	// ciphertext and the key ID of the DEK used. The caller must store
	// ciphertext in the audit/MMR layer; keyID is used later for shredding.
	Encrypt(ctx context.Context, plaintext []byte, tenantID string) (ciphertext []byte, keyID string, err error)

	// ShredKey destroys the DEK identified by keyID. After ShredKey returns
	// successfully, any ciphertext produced under that DEK is permanently
	// unrecoverable (Right to Erasure). The MMR leaf and EnvelopeHash remain
	// valid for inclusion proofs; only decryption becomes impossible.
	ShredKey(ctx context.Context, keyID string) error
}
