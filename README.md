# 🛡️ Vericore OS 

**The Cryptographic Containment Field for Autonomous AI Agents.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vericore/v3r1c0r3)](https://golang.org/)
[![NIST Post-Quantum](https://img.shields.io/badge/Cryptography-ML--DSA%20(Dilithium3)-blue)](https://csrc.nist.gov/projects/post-quantum-cryptography)

Vericore OS is an open-source, API-first compliance primitive designed to make enterprise AI agents legally insurable under the EU AI Act (2026). It physically and cryptographically forces autonomous systems to behave.

## ⚠️ The Problem
LLMs hallucinate. If your AI agent wires $5M to the wrong vendor or hallucinated a diagnosis, "good prompt engineering" is not a legal defense. You need mathematical proof of intent, hardware-backed intervention, and post-quantum audit trails.

## 🚀 Architecture Highlights

* **5,000+ RPS LibSQL Engine:** Bypasses standard SQLite concurrency limits using asynchronous Write-Ahead Batching. Zero `SQLITE_BUSY` locks.
* **Post-Quantum Merkle Ledger:** Every AI action is hashed into an immutable Merkle Mountain Range (MMR) and stamped with a Cloudflare CIRCL Dilithium3 (ML-DSA) signature. Safe from Shore's Algorithm.
* **FIDO2 FinOps Interceptor:** High-stakes API requests (e.g., transfers > $1M) are physically halted by the database until a human signs the transaction with a hardware YubiKey.
* **Confidential Compute (TEE):** Built-in remote attestation endpoints to prove the Go monolith is running inside a secure hardware enclave (AWS Nitro / Intel TDX).
* **Causal Swarm DAG:** Natively orchestrates multi-agent swarms using a Directed Acyclic Graph. Instantly maps and halts the blast radius of an upstream AI hallucination.

## ⚡ Quickstart

Secure your AI in 3 lines of code using the Node SDK.

```bash
npm install @vericore/node-sdk

```

```typescript
import { VericoreClient } from '@vericore/node-sdk';

const client = new VericoreClient(process.env.VERICORE_API_KEY);

// The AI intent is mathematically sealed in the Merkle Tree.
// If amount > threshold, the system physically halts for FIDO2 approval.
const receipt = await client.executeAction({
  agent_id: "agent_treasury_01",
  intent: "wire_funds",
  payload_json: { amount: 5000000, vendor: "Acme Corp" }
});

console.log(`Action Logged. PQC Proof: ${receipt.pqc_signature}`);

```

## 🏗️ Bare-Metal Deployment

Vericore OS bypasses AWS PaaS markups. It is designed for bare-metal deployment using Kamal, secured at the kernel level via Tetragon eBPF, and observable via OpenTelemetry.

```bash
kamal deploy -d api
kamal deploy -d web

```

## 📖 Documentation

See the `/docs` folder for deep dives into the LibSQL Causal Consistency model, the ZKP HealthTech enclave setup, and the Webhook Delivery Engine.
