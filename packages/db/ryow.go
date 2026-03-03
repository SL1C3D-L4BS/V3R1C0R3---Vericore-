package db

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"
)

// ErrReplicaStale is returned by WaitForCommit when the replica has not
// reached the expected state within the allowed timeout. The upstream worker
// should use this to trigger a direct read against the primary DB (e.g. via
// ExecuteWithFallback).
var ErrReplicaStale = errors.New("replica did not sync in time")

// WaitForCommit blocks until the local replica has synced the verification_queue
// record identified by recordID to expectedState, or until the context is
// cancelled or the timeout is reached.
//
// It uses explicit state-polling (Option 12.b) rather than LSN, since the raw
// LibSQL replication LSN may not be exposed by the database/sql driver. The
// function polls verification_queue with exponential backoff (10ms initial,
// 50ms cap) to avoid hammering the local SQLite file, and respects the
// context deadline (e.g. 500ms max). If the timeout is reached before the
// record reaches expectedState, it returns ErrReplicaStale.
func WaitForCommit(ctx context.Context, replicaDB *sql.DB, recordID string, expectedState string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	const (
		initialBackoff = 10 * time.Millisecond
		maxBackoff     = 50 * time.Millisecond
	)

	backoff := initialBackoff
	for {
		if err := ctx.Err(); err != nil {
			return ErrReplicaStale
		}

		var state string
		err := replicaDB.QueryRowContext(ctx,
			`SELECT state FROM verification_queue WHERE id = ?`,
			recordID,
		).Scan(&state)

		if err == nil && state == expectedState {
			return nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		// Not yet synced or not found: log and sleep with backoff then retry.
		got := interface{}(state)
		if err != nil {
			got = err
		}
		log.Printf("ryow: replica lagging, sleeping for %v (record=%s want=%s got=%v)", backoff, recordID, expectedState, got)
		select {
		case <-ctx.Done():
			return ErrReplicaStale
		case <-time.After(backoff):
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}
}

// ExecuteWithFallback ensures read-your-writes consistency for a high-stakes
// job: it first waits for the replica to sync the verification_queue record
// to expectedState. If WaitForCommit returns ErrReplicaStale (timeout), it
// runs execFn against the primary DB. Otherwise it runs execFn against the
// replica. This allows the execution worker to proceed on the primary when
// the replica has not caught up in time, while preferring the replica when
// it has.
func ExecuteWithFallback(ctx context.Context, replica *sql.DB, primary *sql.DB, recordID, expectedState string, timeout time.Duration, execFn func(db *sql.DB) error) error {
	err := WaitForCommit(ctx, replica, recordID, expectedState, timeout)
	if errors.Is(err, ErrReplicaStale) {
		log.Printf("ryow: ErrReplicaStale caught, falling back to primary sqld")
		return execFn(primary)
	}
	if err != nil {
		return err
	}
	log.Printf("ryow: replica synced, executing on edge node")
	return execFn(replica)
}
