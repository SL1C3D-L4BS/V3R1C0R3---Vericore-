package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"v3r1c0r3.local/auth"
	"v3r1c0r3.local/db"
)

// swarmLineageResponse is the JSON array of downstream action hashes and intents (blast radius).
type swarmLineageResponse struct {
	Lineage []swarmLineageEntry `json:"lineage"`
}

type swarmLineageEntry struct {
	Hash   string `json:"hash"`
	Intent string `json:"intent"`
}

// swarmLineageHandler returns the causal lineage (root + all downstream leaves) for GET /api/v1/swarm/lineage/{hash}.
// It uses GetCausalLineage and enriches with intent from audit_event_intents.
func swarmLineageHandler(store *db.LibsqlStore, primary *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hash := chi.URLParam(r, "hash")
		if hash == "" {
			http.Error(w, "hash required", http.StatusBadRequest)
			return
		}
		tenantID := auth.TenantIDFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "tenant context required", http.StatusUnauthorized)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		nodes, err := store.GetCausalLineage(ctx, hash, tenantID)
		if err != nil {
			http.Error(w, "lineage lookup failed", http.StatusInternalServerError)
			return
		}
		if len(nodes) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"lineage":[]}`))
			return
		}

		eventIDs := make([]string, len(nodes))
		for i := range nodes {
			eventIDs[i] = nodes[i].EventID
		}
		intentByEventID, err := getIntentsForEventIDs(ctx, primary, eventIDs, tenantID)
		if err != nil {
			http.Error(w, "intent lookup failed", http.StatusInternalServerError)
			return
		}

		lineage := make([]swarmLineageEntry, len(nodes))
		for i := range nodes {
			lineage[i] = swarmLineageEntry{
				Hash:   nodes[i].HashHex,
				Intent: intentByEventID[nodes[i].EventID],
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(swarmLineageResponse{Lineage: lineage}); err != nil {
			http.Error(w, "encode failed", http.StatusInternalServerError)
			return
		}
	}
}

// getIntentsForEventIDs returns a map of event_id -> intent for the given event IDs and tenant.
func getIntentsForEventIDs(ctx context.Context, d *sql.DB, eventIDs []string, tenantID string) (map[string]string, error) {
	if len(eventIDs) == 0 {
		return nil, nil
	}
	// Build placeholders for IN clause: one per event_id.
	args := make([]interface{}, 0, len(eventIDs)+1)
	args = append(args, tenantID)
	for _, id := range eventIDs {
		args = append(args, id)
	}
	var placeholders string
	for i := 0; i < len(eventIDs); i++ {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
	}
	q := `SELECT event_id, intent FROM audit_event_intents WHERE tenant_id = ? AND event_id IN (` + placeholders + `)`
	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var eid, intent string
		if err := rows.Scan(&eid, &intent); err != nil {
			return nil, err
		}
		out[eid] = intent
	}
	return out, rows.Err()
}
