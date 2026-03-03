import Link from "next/link";

export default function Home() {
  return (
    <main style={{ padding: "2rem", maxWidth: "48rem" }}>
      <h1 style={{ marginBottom: "1rem" }}>V3R1C0R3</h1>
      <p style={{ marginBottom: "1rem" }}>
        EU AI Act Article 14 double-verification and audit.
      </p>
      <Link href="/verification" style={{ color: "inherit", fontWeight: 600 }}>
        → Verification queue
      </Link>
    </main>
  );
}
