"use client";

import { useCallback, useState } from "react";
import { VericoreClient, VericoreGuardrailError } from "@vericore/node-sdk";

const CONFIRMATION_PHRASE = "I APPROVE THIS ACTION";

const vericoreClient = new VericoreClient({
  apiKey: "sk_test_123",
  baseUrl: "http://localhost:8080/api/v1",
});

function bufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

/** Combines authenticatorData and signature into one base64 blob for fido_signature. */
function encodeFidoSignature(authenticatorData: ArrayBuffer, signature: ArrayBuffer): string {
  const a = new Uint8Array(authenticatorData);
  const s = new Uint8Array(signature);
  const combined = new Uint8Array(a.length + s.length);
  combined.set(a);
  combined.set(s, a.length);
  return bufferToBase64(combined.buffer);
}

interface QueueItem {
  id: string;
  actionId: string;
  summary: string;
  state: "pending" | "approved" | "rejected";
}

const MOCK_QUEUE: QueueItem[] = [
  { id: "1", actionId: "act-001", summary: "Deploy model v2 to production", state: "pending" },
  { id: "2", actionId: "act-002", summary: "Update high-stakes prompt template", state: "pending" },
];

export default function VerificationPage() {
  const [confirmationText, setConfirmationText] = useState("");
  const [queue, setQueue] = useState<QueueItem[]>(MOCK_QUEUE);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [status, setStatus] = useState<{ type: "idle" | "loading" | "ok" | "error"; message?: string }>({ type: "idle" });
  const [reasoning, setReasoning] = useState("");

  const canApprove =
    confirmationText.trim() === CONFIRMATION_PHRASE &&
    selectedId != null &&
    queue.find((q) => q.id === selectedId)?.state === "pending";

  const requestWebAuthn = useCallback(async (): Promise<{ authenticatorData: ArrayBuffer; signature: ArrayBuffer } | null> => {
    const challenge = new Uint8Array(32);
    crypto.getRandomValues(challenge);
    try {
      const cred = await navigator.credentials.get({
        publicKey: {
          challenge,
          rpId: window.location.hostname || "localhost",
          timeout: 60000,
          userVerification: "required",
        },
      });
      if (!cred || !("response" in cred)) return null;
      const res = cred.response as AuthenticatorAssertionResponse;
      return {
        authenticatorData: res.authenticatorData,
        signature: res.signature,
      };
    } catch (e) {
      setStatus({ type: "error", message: (e as Error).message });
      return null;
    }
  }, []);

  const submitApproval = useCallback(async () => {
    if (!selectedId || !canApprove) return;
    const item = queue.find((q) => q.id === selectedId);
    if (!item) return;

    setStatus({ type: "loading" });
    const assertion = await requestWebAuthn();
    if (!assertion) return;

    const fidoSignature = encodeFidoSignature(
      assertion.authenticatorData,
      assertion.signature
    );

    const payload = {
      action_id: item.actionId,
      decision: "APPROVED" as const,
      reasoning: reasoning.trim() || "Approved via double-verification UI.",
      fido_signature: fidoSignature,
      record_id: item.id,
      expected_state: "committed",
    };

    try {
      await vericoreClient.executeAction(payload);
      setStatus({ type: "ok", message: "Action approved and recorded." });
      setQueue((prev) =>
        prev.map((q) => (q.id === selectedId ? { ...q, state: "approved" as const } : q))
      );
      setConfirmationText("");
      setReasoning("");
    } catch (e) {
      if (e instanceof VericoreGuardrailError) {
        setStatus({ type: "error", message: `Guardrail blocked: ${e.message}` });
      } else {
        setStatus({ type: "error", message: (e as Error).message });
      }
    }
  }, [selectedId, canApprove, queue, reasoning, requestWebAuthn]);

  return (
    <main style={{ padding: "2rem", maxWidth: "56rem" }}>
      <h1 style={{ marginBottom: "0.5rem" }}>Article 14 Double-Verification Queue</h1>
      <p style={{ marginBottom: "1.5rem", color: "#666" }}>
        Automation bias friction: type the confirmation phrase before approving.
      </p>

      <section style={{ marginBottom: "1.5rem" }}>
        <h2 style={{ marginBottom: "0.75rem", fontSize: "1rem" }}>Pending items</h2>
        <ul style={{ listStyle: "none" }}>
          {queue.map((item) => (
            <li
              key={item.id}
              style={{
                padding: "0.75rem 1rem",
                border: "1px solid #ddd",
                borderRadius: "6px",
                marginBottom: "0.5rem",
                background: selectedId === item.id ? "#f0f7ff" : undefined,
              }}
            >
              <label style={{ display: "flex", alignItems: "center", gap: "0.5rem", cursor: "pointer" }}>
                <input
                  type="radio"
                  name="selected"
                  checked={selectedId === item.id}
                  onChange={() => setSelectedId(item.id)}
                  disabled={item.state !== "pending"}
                />
                <span>
                  <strong>{item.actionId}</strong> — {item.summary}
                  {item.state !== "pending" && (
                    <span style={{ marginLeft: "0.5rem", color: "#0a0", fontWeight: 600 }}>
                      ({item.state})
                    </span>
                  )}
                </span>
              </label>
            </li>
          ))}
        </ul>
      </section>

      <section style={{ marginBottom: "1.5rem" }}>
        <label style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600 }}>
          Reasoning (required)
        </label>
        <textarea
          value={reasoning}
          onChange={(e) => setReasoning(e.target.value)}
          placeholder="Brief justification for this approval..."
          rows={2}
          style={{ width: "100%", padding: "0.5rem", fontSize: "1rem" }}
        />
      </section>

      <section style={{ marginBottom: "1.5rem" }}>
        <label style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600 }}>
          Type &quot;{CONFIRMATION_PHRASE}&quot; to enable Approve
        </label>
        <input
          type="text"
          value={confirmationText}
          onChange={(e) => setConfirmationText(e.target.value)}
          placeholder={CONFIRMATION_PHRASE}
          style={{ width: "100%", padding: "0.5rem", fontSize: "1rem" }}
          aria-label="Confirmation phrase"
        />
      </section>

      <button
        type="button"
        onClick={submitApproval}
        disabled={!canApprove || status.type === "loading"}
        style={{
          padding: "0.6rem 1.2rem",
          fontSize: "1rem",
          fontWeight: 600,
          cursor: canApprove && status.type !== "loading" ? "pointer" : "not-allowed",
          opacity: canApprove && status.type !== "loading" ? 1 : 0.6,
        }}
      >
        {status.type === "loading" ? "Requesting WebAuthn…" : "Approve (WebAuthn)"}
      </button>

      {status.type === "ok" && (
        <p style={{ marginTop: "1rem", color: "#0a0" }}>{status.message}</p>
      )}
      {status.type === "error" && (
        <p style={{ marginTop: "1rem", color: "#c00" }}>{status.message}</p>
      )}
    </main>
  );
}
