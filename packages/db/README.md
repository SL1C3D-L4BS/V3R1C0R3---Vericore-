# packages/db

Database schema, migrations, and client code for **SL1C3D-L4BS - V3R1C0R3 (Vericore)**.

Planned responsibilities (see plan §2):

- LibSQL schema and migrations (Expand-Contract only).
- Audit tables with Article 12–aligned fields and MMR metadata.
- Verification queue table for Article 14 state machine.
- KEK/DEK envelope-encryption metadata for GDPR cryptographic shredding.
- LibSQL client with WAL, dual connection pools, and LSN-based causal consistency helpers.

