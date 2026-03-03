package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const (
	mmrMetaKeyLastExportedIndex = "last_exported_index"
)

// TieredStorageManager orchestrates Hot (LibSQL) -> Warm (ClickHouse) -> Cold (S3 via ClickHouse volume).
// The API strictly controls moves to avoid races with MMR proofs: tombstone is computed and persisted
// before any ALTER MOVE PARTITION.
type TieredStorageManager struct {
	libsql   *sql.DB
	clickhouse *sql.DB
}

// NewTieredStorageManager returns a manager that uses the given LibSQL primary for Hot and
// the ClickHouse connection for Warm/Cold. clickhouseDSN is passed to clickhouse.OpenDB; pass nil
// to use default (e.g. "clickhouse://localhost:9000/default").
func NewTieredStorageManager(libsql *sql.DB, clickhouseOpts *clickhouse.Options) (*TieredStorageManager, error) {
	if clickhouseOpts == nil {
		clickhouseOpts = &clickhouse.Options{Addr: []string{"127.0.0.1:9000"}}
	}
	chDB := clickhouse.OpenDB(clickhouseOpts)
	if err := chDB.PingContext(context.Background()); err != nil {
		_ = chDB.Close()
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}
	if err := ensureTombstonesSchema(context.Background(), libsql); err != nil {
		_ = chDB.Close()
		return nil, err
	}
	return &TieredStorageManager{libsql: libsql, clickhouse: chDB}, nil
}

// Ensure mmr_tombstones exists in LibSQL for Step 2 of ArchivePartitionToCold.
func ensureTombstonesSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS mmr_tombstones (
		partition_month TEXT PRIMARY KEY,
		tombstone_hash  BLOB NOT NULL,
		created_at      DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`)
	return err
}

// ExportToClickHouse reads the oldest unexported mmr_leaves from LibSQL (by mmr_index) and
// bulk-inserts them into ClickHouse audit_log. It updates mmr_meta last_exported_index after a
// successful batch. Async from the caller's perspective: run in a goroutine or worker.
func (m *TieredStorageManager) ExportToClickHouse(ctx context.Context, batchSize int) (exported int, err error) {
	if batchSize <= 0 {
		batchSize = 500
	}

	var lastExported int64
	err = m.libsql.QueryRowContext(ctx, `SELECT next_index FROM mmr_meta WHERE k = ?`, mmrMetaKeyLastExportedIndex).Scan(&lastExported)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("read last_exported_index: %w", err)
	}
	if err == sql.ErrNoRows {
		_, _ = m.libsql.ExecContext(ctx, `INSERT OR IGNORE INTO mmr_meta (k, next_index) VALUES (?, 0)`, mmrMetaKeyLastExportedIndex)
		lastExported = 0
	}

	rows, err := m.libsql.QueryContext(ctx,
		`SELECT id, mmr_index, event_id, hash, tenant_id FROM mmr_leaves WHERE mmr_index > ? ORDER BY mmr_index ASC LIMIT ?`,
		lastExported, batchSize)
	if err != nil {
		return 0, fmt.Errorf("query mmr_leaves: %w", err)
	}
	defer rows.Close()

	type leafRow struct {
		id       string
		mmrIndex uint64
		eventID  string
		hash     []byte
		tenantID string
	}
	var batch []leafRow
	for rows.Next() {
		var r leafRow
		if err := rows.Scan(&r.id, &r.mmrIndex, &r.eventID, &r.hash, &r.tenantID); err != nil {
			return 0, fmt.Errorf("scan mmr_leaf: %w", err)
		}
		if r.tenantID == "" {
			r.tenantID = "default"
		}
		batch = append(batch, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(batch) == 0 {
		return 0, nil
	}

	tx, err := m.clickhouse.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("clickhouse begin: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO audit_log (id, mmr_index, event_id, hash, tenant_id, ingested_at) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("clickhouse prepare: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, r := range batch {
		_, err := stmt.ExecContext(ctx, r.id, r.mmrIndex, r.eventID, hex.EncodeToString(r.hash), r.tenantID, now)
		if err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("clickhouse exec row: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("clickhouse commit: %w", err)
	}

	newLast := lastExported
	for _, r := range batch {
		if uint64(newLast) < r.mmrIndex {
			newLast = int64(r.mmrIndex)
		}
	}
	_, err = m.libsql.ExecContext(ctx, `INSERT OR REPLACE INTO mmr_meta (k, next_index) VALUES (?, ?)`, mmrMetaKeyLastExportedIndex, newLast)
	if err != nil {
		return len(batch), fmt.Errorf("update last_exported_index: %w", err)
	}
	log.Printf("tiered_storage: exported %d mmr_leaves to ClickHouse (last_exported_index=%d)", len(batch), newLast)
	return len(batch), nil
}

// ArchivePartitionToCold moves a ClickHouse partition to the 'cold' volume only after
// computing and persisting the Tombstone Hash in LibSQL (no TTL race with MMR proofs).
// partitionMonth is the partition id, e.g. "202603" for March 2026.
func (m *TieredStorageManager) ArchivePartitionToCold(ctx context.Context, partitionMonth string) error {
	// Step 1: Compute tombstone hash from all records in the partition (MMR leaves in order).
	rows, err := m.clickhouse.QueryContext(ctx,
		`SELECT id, mmr_index, event_id, hash FROM audit_log WHERE toYYYYMM(ingested_at) = ? ORDER BY mmr_index`,
		partitionMonth)
	if err != nil {
		return fmt.Errorf("query partition for tombstone: %w", err)
	}
	defer rows.Close()

	h := sha256.New()
	var count int
	for rows.Next() {
		var id, eventID, hashStr string
		var mmrIndex uint64
		if err := rows.Scan(&id, &mmrIndex, &eventID, &hashStr); err != nil {
			return fmt.Errorf("scan partition row: %w", err)
		}
		h.Write([]byte(id))
		h.Write([]byte(eventID))
		h.Write([]byte(hashStr))
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("partition %s: no rows (nothing to archive)", partitionMonth)
	}
	tombstoneHash := h.Sum(nil)

	// Step 2: Persist tombstone in LibSQL before moving data.
	_, err = m.libsql.ExecContext(ctx,
		`INSERT OR REPLACE INTO mmr_tombstones (partition_month, tombstone_hash, created_at) VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'))`,
		partitionMonth, tombstoneHash)
	if err != nil {
		return fmt.Errorf("persist tombstone: %w", err)
	}
	log.Printf("tiered_storage: tombstone for partition %s persisted (hash=%s, rows=%d)", partitionMonth, hex.EncodeToString(tombstoneHash)[:16], count)

	// Step 3: Move partition to cold volume. (partitionMonth must be YYYYMM; validated to avoid injection.)
	if len(partitionMonth) != 6 || !isDigits(partitionMonth) {
		return fmt.Errorf("partition_month must be 6 digits (YYYYMM), got %q", partitionMonth)
	}
	_, err = m.clickhouse.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE audit_log MOVE PARTITION '%s' TO VOLUME 'cold'`, partitionMonth))
	if err != nil {
		return fmt.Errorf("move partition to cold: %w", err)
	}
	log.Printf("tiered_storage: partition %s moved to volume 'cold'", partitionMonth)
	return nil
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Close releases the ClickHouse connection. LibSQL is not closed (caller owns it).
func (m *TieredStorageManager) Close() error {
	if m.clickhouse != nil {
		return m.clickhouse.Close()
	}
	return nil
}
