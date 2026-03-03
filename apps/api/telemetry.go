package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

const intentInterventionBlocked = "guardrail_intervention_blocked"

// EnsureAuditEventIntentsTable creates the audit_event_intents table if it does not exist.
// Call at startup so Article 72 telemetry can record event_id and intent per append.
func EnsureAuditEventIntentsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS audit_event_intents (
		event_id   TEXT PRIMARY KEY,
		intent     TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		tenant_id  TEXT NOT NULL DEFAULT 'default'
	)`)
	return err
}

// RecordAuditIntent records an event's intent for telemetry (Article 72). Idempotent: INSERT OR IGNORE.
func RecordAuditIntent(ctx context.Context, db *sql.DB, eventID, intent, tenantID string) {
	if tenantID == "" {
		tenantID = "default"
	}
	_, _ = db.ExecContext(ctx,
		`INSERT OR IGNORE INTO audit_event_intents (event_id, intent, created_at, tenant_id) VALUES (?, ?, ?, ?)`,
		eventID, intent, time.Now().UTC().Format(time.RFC3339), tenantID)
}

// telemetryStatsResponse is the JSON payload for GET /api/v1/telemetry/stats.
type telemetryStatsResponse struct {
	TotalEvents           int64                `json:"total_events"`
	InterventionsBlocked  int64                `json:"interventions_blocked"`
	DailyTimeseries       []dailyTelemetryPoint `json:"daily_timeseries"`
}

type dailyTelemetryPoint struct {
	Date            string `json:"date"`
	ApprovedActions int64  `json:"approved_actions"`
	BlockedActions  int64  `json:"blocked_actions"`
}

// telemetryStatsHandler returns aggregated stats from the primary DB for the compliance dashboard.
func telemetryStatsHandler(primary *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var totalEvents int64
		err := primary.QueryRowContext(ctx, `SELECT next_index FROM mmr_meta WHERE k = 'next'`).Scan(&totalEvents)
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "failed to read total_events", http.StatusInternalServerError)
			return
		}

		var interventionsBlocked int64
		_ = primary.QueryRowContext(ctx, `SELECT count(*) FROM audit_event_intents WHERE intent = ?`, intentInterventionBlocked).Scan(&interventionsBlocked)

		rows, err := primary.QueryContext(ctx, `SELECT
			date(created_at) AS day,
			sum(CASE WHEN intent = ? THEN 1 ELSE 0 END) AS blocked,
			sum(CASE WHEN intent != ? THEN 1 ELSE 0 END) AS approved
		FROM audit_event_intents
		WHERE created_at >= date('now', '-7 days')
		GROUP BY date(created_at)
		ORDER BY day`, intentInterventionBlocked, intentInterventionBlocked)
		if err != nil {
			http.Error(w, "failed to read daily timeseries", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var daily []dailyTelemetryPoint
		for rows.Next() {
			var p dailyTelemetryPoint
			if err := rows.Scan(&p.Date, &p.BlockedActions, &p.ApprovedActions); err != nil {
				http.Error(w, "failed to scan daily row", http.StatusInternalServerError)
				return
			}
			daily = append(daily, p)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "daily timeseries error", http.StatusInternalServerError)
			return
		}

		resp := telemetryStatsResponse{
			TotalEvents:          totalEvents,
			InterventionsBlocked: interventionsBlocked,
			DailyTimeseries:      daily,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
