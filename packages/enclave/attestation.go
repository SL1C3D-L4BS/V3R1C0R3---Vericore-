package enclave

import (
	"crypto/sha512"
	"encoding/hex"
	"errors"

	"v3r1c0r3.local/pqc"
)

var errSignFailed = errors.New("enclave: PQC sign failed")

// Measurement holds the attested platform configuration (mock PCR0) and its PQC signature.
type Measurement struct {
	PCR0             string `json:"pcr0"`
	EnclaveSignature []byte `json:"enclave_signature"`
}

const mockPCR0Input = "mock-pcr0-sha384-hash"

// GenerateAttestationReport produces a mock TEE measurement: SHA-384 of a fixed string as PCR0,
// signed with the application's PQC private key to prove the measurement comes from the verified app layer.
// In production, PCR0 would be supplied by hardware (e.g. Intel TDX / AMD SEV-SNP).
func GenerateAttestationReport(pqcPrivKey []byte) (Measurement, error) {
	h := sha512.Sum384([]byte(mockPCR0Input))
	pcr0Hex := hex.EncodeToString(h[:])
	sig := pqc.Sign(pqcPrivKey, h[:])
	if sig == nil {
		return Measurement{}, errSignFailed
	}
	return Measurement{PCR0: pcr0Hex, EnclaveSignature: sig}, nil
}
