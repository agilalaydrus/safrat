"use client";

export default function PilgrimsError({
  error,
  reset,
}: {
  error: Error;
  reset: () => void;
}) {
  return (
    <main style={{ maxWidth: 600, margin: "80px auto", padding: "0 24px", textAlign: "center" }}>
      <p style={{ color: "var(--color-gold-800)", fontWeight: 700 }}>Failed to load pilgrims</p>
      <p style={{ color: "var(--color-warm-500)", fontSize: 14 }}>{error.message}</p>
      <button onClick={reset} style={{ minHeight: 48, padding: "0 20px", background: "var(--color-emerald-900)", color: "white", border: 0, borderRadius: 8, fontWeight: 700, marginTop: 16 }}>
        Try again
      </button>
    </main>
  );
}
