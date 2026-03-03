//! RISC Zero zkVM guest: cryptographic blinding for Article 14 / ZKP pipeline.
//! PrivateInput is read from the environment; only a hash and the policy decision
//! are committed to the journal (no PII).

#![no_main]
#![no_std]

risc0_zkvm::entry!(main);

#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    risc0_zkvm::guest::env::exit(1)
}

use risc0_zkvm::guest::env;
use risc0_zkvm::sha::{Impl, Sha256};

/// Private input layout (must match host serialization):
/// - trade_amount: u64 (LE)
/// - payload_len: u32 (LE) = length of user_id + sensitive_context blob
/// - payload: [u8; payload_len]
///
/// The guest never commits plaintext; it hashes the full input and commits
/// only the decision and input_hash (PublicReceipt).

fn main() {
    // Read private input: amount then variable-length payload
    let trade_amount: u64 = env::read();
    let payload_len: u32 = env::read();
    let mut payload = [0u8; 64 * 1024];
    let len = payload_len as usize;
    if len > payload.len() {
        env::exit(1);
    }
    env::read_slice(&mut payload[..len]);

    // Build canonical blob for hashing: amount (8) + len (4) + payload
    let mut input_for_hash = [0u8; 64 * 1024 + 12];
    input_for_hash[0..8].copy_from_slice(&trade_amount.to_le_bytes());
    input_for_hash[8..12].copy_from_slice(&payload_len.to_le_bytes());
    input_for_hash[12..12 + len].copy_from_slice(&payload[..len]);
    let input_len = 12 + len;

    // Blinding step: hash entire PrivateInput (SHA-256)
    let input_hash = Impl::hash_bytes(&input_for_hash[..input_len]);

    // High-stakes policy: no approval above threshold
    let decision = trade_amount <= 10_000;

    // Public receipt: ONLY decision and input_hash (never plaintext)
    let mut journal = [0u8; 33];
    journal[0] = if decision { 1 } else { 0 };
    journal[1..33].copy_from_slice(input_hash.as_bytes());
    env::commit_slice(&journal);
}
