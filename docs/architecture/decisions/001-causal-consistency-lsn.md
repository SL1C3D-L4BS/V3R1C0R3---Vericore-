# ADR 001: Causal Consistency via LSN for Article 14 Double-Verification

## Context and Problem Statement
Multi-actor double-verification workflows (EU AI Act Article 14) create a replication race condition. When Approver 2 commits a state change to the self-hosted sqld primary, the execution worker (running on a distributed bare-metal node) may read from an embedded LibSQL replica that has not yet synced, leading to false anomalies or dropped executions.

## Decision Drivers
- Must support distributed nodes under BGP Anycast.
- Must guarantee execution only happens on cryptographically verified state.
- Time-based polling fallbacks are unreliable and unacceptable.

## Considered Options
1. Direct primary reads for all high-stakes actions.
2. Token-based causal consistency (LSN wait).

## Decision Outcome
Chosen option: **Token-based causal consistency (LSN wait)**. Upon the final approval commit, the primary issues a Log Sequence Number (LSN). The execution worker receives this token and strictly blocks execution until its local embedded replica confirms synchronization up to or beyond this LSN. If a 500ms timeout occurs, it falls back to a primary read.

## Consequences
- Good: Eliminates replication race conditions; maintains edge-node efficiency.
- Bad: Adds complexity to the worker queue payload and requires robust timeout handling in the Go API.
