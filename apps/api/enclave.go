package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"v3r1c0r3.local/enclave"
)

// attestResponse is the JSON response for GET /api/v1/enclave/attest.
// EnclaveSignature is base64-encoded for JSON.
type attestResponse struct {
	PCR0             string `json:"pcr0"`
	EnclaveSignature string `json:"enclave_signature"`
}

// enclaveAttestHandler handles GET /api/v1/enclave/attest (public, no auth).
// Returns the TEE measurement and PQC-signed PCR0 for remote attestation.
func enclaveAttestHandler(pqcPrivKey []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m, err := enclave.GenerateAttestationReport(pqcPrivKey)
		if err != nil {
			http.Error(w, "attestation failed", http.StatusInternalServerError)
			return
		}
		out := attestResponse{
			PCR0:             m.PCR0,
			EnclaveSignature: base64.StdEncoding.EncodeToString(m.EnclaveSignature),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}
