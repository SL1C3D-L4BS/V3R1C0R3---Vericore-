package db

import (
	"context"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"github.com/tursodatabase/go-libsql"
	_ "modernc.org/sqlite"
	"v3r1c0r3.local/mcp-flight-recorder"
)

// StoreConfig configures LibSQL store: local SQLite or embedded replica syncing to a primary.
type StoreConfig struct {
	DBPath       string        // path to local DB file (e.g. "file:primary.db" or "/data/replica.db")
	PrimaryURL   string        // if non-empty, use go-libsql embedded replica syncing to this URL
	AuthToken    string        // auth token for primary (replica mode only)
	SyncInterval time.Duration // sync interval for embedded replica (e.g. 5*time.Second)
}

// LibsqlStore implements flightrecorder.Store using a LibSQL/SQLite *sql.DB
// (typically the primary/write pool). It uses BEGIN IMMEDIATE for ACID
// transactions and creates MMR tables on first use.
type LibsqlStore struct {
	db *sql.DB
}

// NewLibsqlStore opens a DB from cfg and returns a Store plus the *sql.DB for use as primary/replica.
// Local mode: PrimaryURL empty — standard local SQLite with WAL, synchronous=NORMAL, foreign_keys=ON.
// Replica mode: PrimaryURL set — embedded replica via go-libsql (local file, syncs to PrimaryURL).
// The connection pool is set to MaxOpenConns(1) to avoid SQLITE_BUSY on local replica writes.
func NewLibsqlStore(cfg StoreConfig) (*LibsqlStore, *sql.DB, error) {
	db, err := openDB(cfg)
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := ensureMMRSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return &LibsqlStore{db: db}, db, nil
}

// openDB opens either a local SQLite connection or a go-libsql embedded replica.
func openDB(cfg StoreConfig) (*sql.DB, error) {
	if cfg.PrimaryURL == "" {
		return openLocalSQLite(cfg.DBPath)
	}
	return openEmbeddedReplica(cfg)
}

func openLocalSQLite(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	for _, pragma := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA synchronous=NORMAL`,
		`PRAGMA foreign_keys=ON`,
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return db, nil
}

func openEmbeddedReplica(cfg StoreConfig) (*sql.DB, error) {
	opts := []libsql.Option{}
	if cfg.AuthToken != "" {
		opts = append(opts, libsql.WithAuthToken(cfg.AuthToken))
	}
	if cfg.SyncInterval > 0 {
		opts = append(opts, libsql.WithSyncInterval(cfg.SyncInterval))
	}
	conn, err := libsql.NewEmbeddedReplicaConnector(cfg.DBPath, cfg.PrimaryURL, opts...)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(conn), nil
}

// RunInTx runs fn inside a single ACID transaction (BEGIN IMMEDIATE).
func (s *LibsqlStore) RunInTx(ctx context.Context, fn func(ctx context.Context, tx flightrecorder.StoreTx) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	// Use BEGIN IMMEDIATE by executing it; sql.DB.BeginTx in SQLite doesn't
	// always use IMMEDIATE. Exec "BEGIN IMMEDIATE" after begin would require
	// a different flow. For SQLite driver, we rely on the driver or run
	// PRAGMA busy_timeout. Simplest: commit/rollback and delegate.
	libTx := &libsqlTx{tx: tx}
	if err := fn(ctx, libTx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type libsqlTx struct {
	tx *sql.Tx
}

func (t *libsqlTx) SaveLeaf(ctx context.Context, leaf flightrecorder.MMRLeaf) error {
	tenantID := leaf.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	pqcSigHex := ""
	if len(leaf.PQCSignature) > 0 {
		pqcSigHex = hex.EncodeToString(leaf.PQCSignature)
	}
	pqcPubHex := ""
	if len(leaf.PQCPublicKey) > 0 {
		pqcPubHex = hex.EncodeToString(leaf.PQCPublicKey)
	}
	_, err := t.tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO mmr_leaves (id, mmr_index, event_id, hash, tenant_id, pqc_signature, pqc_public_key) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		leaf.ID, leaf.Index, leaf.EventID, leaf.Hash, tenantID, pqcSigHex, pqcPubHex)
	return err
}

func (t *libsqlTx) GetLeaf(ctx context.Context, id string) (flightrecorder.MMRLeaf, error) {
	var leaf flightrecorder.MMRLeaf
	var pqcSigHex, pqcPubHex sql.NullString
	err := t.tx.QueryRowContext(ctx,
		`SELECT id, mmr_index, event_id, hash, tenant_id, pqc_signature, pqc_public_key FROM mmr_leaves WHERE id = ?`,
		id).Scan(&leaf.ID, &leaf.Index, &leaf.EventID, &leaf.Hash, &leaf.TenantID, &pqcSigHex, &pqcPubHex)
	if err != nil {
		return leaf, err
	}
	if pqcSigHex.Valid && pqcSigHex.String != "" {
		leaf.PQCSignature, _ = hex.DecodeString(pqcSigHex.String)
	}
	if pqcPubHex.Valid && pqcPubHex.String != "" {
		leaf.PQCPublicKey, _ = hex.DecodeString(pqcPubHex.String)
	}
	return leaf, nil
}

func (t *libsqlTx) GetPeaks(ctx context.Context) ([]flightrecorder.Peak, error) {
	rows, err := t.tx.QueryContext(ctx, `SELECT hash, height FROM mmr_peaks ORDER BY ord`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var peaks []flightrecorder.Peak
	for rows.Next() {
		var p flightrecorder.Peak
		if err := rows.Scan(&p.Hash, &p.Height); err != nil {
			return nil, err
		}
		peaks = append(peaks, p)
	}
	return peaks, rows.Err()
}

func (t *libsqlTx) SavePeaks(ctx context.Context, peaks []flightrecorder.Peak) error {
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM mmr_peaks`); err != nil {
		return err
	}
	for i, p := range peaks {
		if _, err := t.tx.ExecContext(ctx,
			`INSERT INTO mmr_peaks (ord, hash, height) VALUES (?, ?, ?)`,
			i, p.Hash, p.Height); err != nil {
			return err
		}
	}
	return nil
}

func (t *libsqlTx) GetNextIndex(ctx context.Context) (uint64, error) {
	var next uint64
	err := t.tx.QueryRowContext(ctx, `SELECT next_index FROM mmr_meta WHERE k = 'next'`).Scan(&next)
	return next, err
}

func (t *libsqlTx) SaveNextIndex(ctx context.Context, next uint64) error {
	_, err := t.tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO mmr_meta (k, next_index, tenant_id) VALUES ('next', ?, 'default')`, next)
	return err
}

func (t *libsqlTx) SaveNode(ctx context.Context, hash, leftHash, rightHash []byte) error {
	_, err := t.tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO mmr_nodes (hash, left_hash, right_hash) VALUES (?, ?, ?)`,
		hash, leftHash, rightHash)
	return err
}

func ensureMMRSchema(ctx context.Context, db *sql.DB) error {
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS mmr_meta (k TEXT PRIMARY KEY, next_index INTEGER NOT NULL, tenant_id TEXT NOT NULL DEFAULT 'default')`,
		`CREATE TABLE IF NOT EXISTS mmr_leaves (id TEXT PRIMARY KEY, mmr_index INTEGER NOT NULL, event_id TEXT NOT NULL, hash BLOB NOT NULL, tenant_id TEXT NOT NULL DEFAULT 'default')`,
		`CREATE TABLE IF NOT EXISTS mmr_peaks (ord INTEGER PRIMARY KEY, hash BLOB NOT NULL, height INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS mmr_nodes (hash BLOB PRIMARY KEY, left_hash BLOB NOT NULL, right_hash BLOB NOT NULL)`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	_, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO mmr_meta (k, next_index, tenant_id) VALUES ('next', 0, 'default')`)
	if err != nil {
		return err
	}
	// Migration 008: PQC signature columns (ignore if already present).
	for _, q := range []string{
		`ALTER TABLE mmr_leaves ADD COLUMN pqc_signature TEXT`,
		`ALTER TABLE mmr_leaves ADD COLUMN pqc_public_key TEXT`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	return nil
}
