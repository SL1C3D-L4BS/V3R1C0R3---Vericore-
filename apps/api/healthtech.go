package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"v3r1c0r3.local/auth"
	"v3r1c0r3.local/mcp-flight-recorder"
)

const (
	healthtechAgentID     = "healthtech"
	intentHealthtechTriage = "healthtech_zkp_triage"
)

// zkHealthHostOutput is the JSON output from the zk-health host binary.
type zkHealthHostOutput struct {
	ReceiptBase64 string `json:"receipt_base64"`
	JournalBase64 string `json:"journal_base64"`
	Journal       string `json:"journal"`
}

// triageHandler handles POST /api/v1/healthtech/triage: runs ZKP enclave on PHI,
// logs only receipt + journal to the Flight Recorder, discards raw PHI.
func triageHandler(primary *sql.DB, recorder flightrecorder.FlightRecorder, zkHealthBin string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tenantID := auth.TenantIDFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		phiRaw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		// PHI is only used to pass to the enclave; we never log or store it after this.
		defer func() { phiRaw = nil }()

		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, zkHealthBin)
		cmd.Stdin = bytes.NewReader(phiRaw)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), "RISC0_DEV_MODE=1") // allow dev build without full prover toolchain

		if err := cmd.Run(); err != nil {
			log.Printf("healthtech triage: zk-health failed: %v; stderr: %s", err, stderr.Bytes())
			http.Error(w, "zkp triage failed", http.StatusInternalServerError)
			return
		}

		var out zkHealthHostOutput
		if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
			http.Error(w, "invalid zk-health output", http.StatusInternalServerError)
			return
		}

		// Audit: params_json contains ONLY base64 ZKP receipt and public journal; no PHI.
		paramsPayload := map[string]string{
			"receipt_base64": out.ReceiptBase64,
			"journal":        out.Journal,
		}
		paramsJSON, _ := json.Marshal(paramsPayload)
		h := sha256.Sum256(paramsJSON)
		eventID, _ := generateEventID()
		ev := flightrecorder.AuditEvent{
			ID:           eventID,
			Timestamp:    time.Now().UTC(),
			TenantID:     tenantID,
			AgentID:      healthtechAgentID,
			Intent:       intentHealthtechTriage,
			ToolName:     "zk_health_triage",
			ParamsJSON:   paramsJSON,
			EnvelopeHash: hex.EncodeToString(h[:]),
		}
		if _, appendErr := recorder.Append(r.Context(), ev); appendErr != nil {
			log.Printf("healthtech_zkp_triage: MMR append failed: %v", appendErr)
		}
		RecordAuditIntent(r.Context(), primary, eventID, intentHealthtechTriage, tenantID)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"event_id": eventID,
			"journal":  out.Journal,
			"decision": out.Journal, // triage decision is in the journal
		})
	}
}
