//! Host CLI for HealthTech ZKP triage. Reads PHI JSON from argv or stdin,
//! runs the RISC Zero prover, outputs JSON with serialized receipt and public journal.

use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use risc0_zkvm::{default_prover, ExecutorEnv};
use serde::Serialize;
use zk_health_methods::{TRIAGE_ELF, TRIAGE_ID};

#[derive(Serialize)]
struct Output {
    receipt_base64: String,
    journal_base64: String,
    journal: String,
}

fn main() {
    let phi_json = read_phi_input();
    let phi_bytes = phi_json.as_bytes();
    let len = phi_bytes.len() as u32;

    let env = ExecutorEnv::builder()
        .write(&len)
        .unwrap()
        .write_slice(phi_bytes)
        .build()
        .unwrap();

    let prover = default_prover();
    let receipt = prover
        .prove(env, TRIAGE_ELF)
        .expect("prove failed");

    receipt.verify(TRIAGE_ID).expect("receipt verify failed");

    let journal_bytes = receipt.journal.bytes;
    let journal_str = String::from_utf8_lossy(&journal_bytes);
    // Serialize receipt for storage; fallback to journal digest if Receipt doesn't serialize
    let receipt_bin = bincode::serialize(&receipt).unwrap_or_else(|_| journal_bytes.clone());

    let out = Output {
        receipt_base64: BASE64.encode(&receipt_bin),
        journal_base64: BASE64.encode(&journal_bytes),
        journal: journal_str.to_string(),
    };
    println!("{}", serde_json::to_string(&out).unwrap());
}

fn read_phi_input() -> String {
    let args: Vec<String> = std::env::args().collect();
    if args.len() > 1 {
        let arg = &args[1];
        if arg == "-" {
            std::io::read_to_string(std::io::stdin()).unwrap_or_default()
        } else {
            std::fs::read_to_string(arg).unwrap_or_else(|_| arg.clone())
        }
    } else {
        std::io::read_to_string(std::io::stdin()).unwrap_or_default()
    }
}
