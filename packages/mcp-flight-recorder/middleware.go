package flightrecorder

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"v3r1c0r3.local/auth"
	"v3r1c0r3.local/pqc"
)

// W3C Trace Context: https://www.w3.org/TR/trace-context/#traceparent-header
const traceparentHeader = "traceparent"

// contextKey type for traceparent to avoid context key collisions.
type contextKey string

const traceparentContextKey contextKey = "traceparent"

// TraceparentFromContext returns the traceparent value stored in ctx, or "" if not set.
func TraceparentFromContext(ctx context.Context) string {
	if v := ctx.Value(traceparentContextKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ActionPayload is the expected shape of a state-changing request body when using
// AuditMiddleware. The middleware extracts intent, tool_name, params, and optional
// context_hash from the request body to construct the AuditEvent.
type ActionPayload struct {
	Intent      string          `json:"intent"`
	ToolName    string          `json:"tool_name"`
	Params      json.RawMessage `json:"params"`
	ContextHash string          `json:"context_hash,omitempty"`
	ParentHash  string          `json:"parent_hash,omitempty"`
}

// AfterAppendFunc is called after a successful recorder.Append with the event ID and intent.
// Used by the API to record event intents for Article 72 telemetry (e.g. audit_event_intents table).
// May be nil.
type AfterAppendFunc func(ctx context.Context, eventID, intent string)

// AuditMiddleware wraps an http.Handler and intercepts every state-changing
// request: it extracts the proposed action (intent, tool, params), builds an
// AuditEvent, and calls recorder.Append. The request proceeds to the domain
// handler only if Append succeeds; otherwise the middleware responds with
// HTTP 500 and does not invoke the next handler. The audit log must never
// drop an event.
//
// The traceparent header (W3C Trace Context) is read from the incoming request
// and injected into the context passed to Append, so that downstream stores
// and observability can maintain trace continuity.
//
// AgentID is set from the optional X-Agent-ID request header; if missing,
// defaultAgentID is used.
//
// If afterAppend is non-nil, it is called after each successful Append with (ctx, event.ID, event.Intent).
//
// If pqcPrivateKey and pqcPublicKey are both non-nil, the event is signed with the post-quantum
// key before Append: canonical event JSON (without PQC fields) is signed, then PQCSignature and
// PQCPublicKey are set on the event so the leaf hash binds the signature.
func AuditMiddleware(recorder FlightRecorder, defaultAgentID string, next http.Handler, afterAppend AfterAppendFunc, pqcPrivateKey, pqcPublicKey []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract traceparent and inject into context.
		traceparent := r.Header.Get(traceparentHeader)
		ctx := context.WithValue(r.Context(), traceparentContextKey, traceparent)

		// 2. Extract proposed action from request body (for state-changing methods).
		var bodyBytes []byte
		if r.Body != nil {
			var err error
			bodyBytes, err = io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
				return
			}
			// Restore body so the domain handler can read it.
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		var payload ActionPayload
		if len(bodyBytes) > 0 {
			_ = json.Unmarshal(bodyBytes, &payload)
		}

		agentID := r.Header.Get("X-Agent-ID")
		if agentID == "" {
			agentID = defaultAgentID
		}

		tenantID := auth.TenantIDFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "tenant context required", http.StatusInternalServerError)
			return
		}

		// 3. Build AuditEvent.
		eventID, err := generateEventID()
		if err != nil {
			http.Error(w, "failed to generate event id", http.StatusInternalServerError)
			return
		}

		paramsJSON := payload.Params
		if paramsJSON == nil {
			paramsJSON = json.RawMessage(bodyBytes)
		}
		envelopeHash := hashForIntegrity(paramsJSON)

		event := AuditEvent{
			ID:           eventID,
			Timestamp:    time.Now().UTC(),
			TenantID:     tenantID,
			AgentID:      agentID,
			Intent:       payload.Intent,
			ToolName:     payload.ToolName,
			ParamsJSON:   paramsJSON,
			EnvelopeHash: envelopeHash,
			ContextHash:  payload.ContextHash,
			ParentHash:   payload.ParentHash,
		}

		// 3b. Post-quantum sign the event (canonical JSON without PQC fields) if keys provided.
		if len(pqcPrivateKey) > 0 && len(pqcPublicKey) > 0 {
			eventForSigning := event
			eventForSigning.PQCSignature = nil
			eventForSigning.PQCPublicKey = nil
			eventJSON, _ := json.Marshal(eventForSigning)
			sig := pqc.Sign(pqcPrivateKey, eventJSON)
			event.PQCSignature = sig
			event.PQCPublicKey = pqcPublicKey
		}

		// 4. Append to MMR; abort request on failure.
		_, err = recorder.Append(ctx, event)
		if err != nil {
			http.Error(w, "audit append failed", http.StatusInternalServerError)
			return
		}
		if afterAppend != nil {
			afterAppend(ctx, event.ID, event.Intent)
		}

		// 5. Audit succeeded; allow request to proceed.
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateEventID returns a new opaque ID for an audit event (e.g. 32-char hex).
func generateEventID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashForIntegrity returns a hex-encoded SHA-256 hash of data for EnvelopeHash.
func hashForIntegrity(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
