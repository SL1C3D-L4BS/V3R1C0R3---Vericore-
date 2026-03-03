//! HealthTech ZKP guest: triage on PHI (PatientProfile). Only the triage decision
//! is committed to the journal; raw patient data stays private inside the guest.

#![no_main]
#![no_std]

#[panic_handler]
fn panic(_: &core::panic::PanicInfo) -> ! {
    risc0_zkvm::guest::env::exit(1);
}

use risc0_zkvm::guest::env;

risc0_zkvm::guest::entry!(main);

/// Triage decision committed to the public journal (no PHI).
#[derive(serde::Serialize)]
struct TriageDecision {
    decision: &'static str,
    reason: &'static str,
}

const MAX_PHI_LEN: usize = 4096;

/// Extract blood_pressure_systolic from JSON bytes (minimal no_std parsing).
/// Returns None if not found or invalid.
fn parse_systolic_from_json(json: &[u8]) -> Option<u32> {
    let s = core::str::from_utf8(json).ok()?;
    let key = "\"blood_pressure_systolic\"";
    let pos = s.find(key)?;
    let after_key = &s[pos + key.len()..];
    let colon = after_key.find(':')?;
    let after_colon = after_key[colon + 1..].trim_start();
    let end = after_colon
        .find(|c: char| !c.is_ascii_digit())
        .unwrap_or(after_colon.len());
    after_colon[..end].parse::<u32>().ok()
}

fn main() {
    // Read PHI JSON from host (private input; never committed). Length-prefixed.
    let len: u32 = env::read();
    let mut buf = [0u8; MAX_PHI_LEN];
    let n = len as usize;
    if n > buf.len() {
        env::exit(1);
    }
    env::read_slice(&mut buf[..n]);
    let phi_bytes = &buf[..n];
    let systolic = parse_systolic_from_json(phi_bytes);

    let decision = match systolic {
        Some(s) if s > 180 => TriageDecision {
            decision: "URGENT_ER_ROUTING",
            reason: "BP_THRESHOLD_EXCEEDED",
        },
        _ => TriageDecision {
            decision: "ROUTINE",
            reason: "BP_WITHIN_LIMITS",
        },
    };

    // Commit only the triage decision to the public journal; PHI stays private.
    env::commit(&decision);
}
