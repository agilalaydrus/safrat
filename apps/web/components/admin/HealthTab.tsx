"use client";

import { useEffect, useState } from "react";
import { IconAlertTriangle, IconCircleCheck, IconEyeOff, IconRefresh } from "@tabler/icons-react";
import type { GetPlatformHealthResponse, HealthSignal } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const TONE: Record<string, { background: string; colour: string; label: string }> = {
  OK: { background: "var(--color-emerald-50)", colour: "var(--color-emerald-800)", label: "Aman" },
  WARN: { background: "var(--color-warning-50)", colour: "var(--color-warning-700)", label: "Perlu dilihat" },
  ALERT: { background: "var(--color-danger-100)", colour: "var(--color-danger-600)", label: "Perlu tindakan" },
  UNMONITORED: { background: "var(--color-cream-200)", colour: "var(--color-warm-700)", label: "Tidak dipantau" },
};

const timeOf = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "";

function StatusIcon({ status }: { status: string }) {
  if (status === "OK") return <IconCircleCheck size={18} />;
  if (status === "UNMONITORED") return <IconEyeOff size={18} />;
  return <IconAlertTriangle size={18} />;
}

export default function HealthTab() {
  const [health, setHealth] = useState<GetPlatformHealthResponse>();
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");

  const load = () => {
    setLoading(true);
    setFailure("");
    platformClient
      .getPlatformHealth({})
      .then(setHealth)
      .catch(() => setFailure("Gagal memuat kesehatan platform."))
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  if (loading) return <p style={muted}>Memeriksa…</p>;
  if (failure) return <p style={errorBox}><IconAlertTriangle size={16} />{failure}</p>;
  if (!health) return null;

  const needsAttention = health.signals.filter((signal) => signal.status === "WARN" || signal.status === "ALERT");
  const unmonitored = health.signals.filter((signal) => signal.status === "UNMONITORED");

  return (
    <section style={{ display: "grid", gap: 18 }}>
      <div style={{ display: "flex", justifyContent: "space-between", gap: 16, flexWrap: "wrap", alignItems: "flex-start" }}>
        <div>
          <h2 style={heading}>Kesehatan Platform</h2>
          <p style={muted}>
            Diperbarui {timeOf(health.checkedAt)} · {needsAttention.length} butir perlu perhatian
            {unmonitored.length > 0 && ` · ${unmonitored.length} tidak dipantau`}
          </p>
          <p style={{ ...muted, fontSize: 12, marginTop: 4 }}>
            Hanya yang berdampak ke pelanggan. Bukan konsol infrastruktur — grafik CPU membuat daftar ini terlalu
            panjang untuk dibaca justru saat ada yang benar-benar rusak.
          </p>
        </div>
        <button type="button" onClick={load} style={refreshButton}>
          <IconRefresh size={15} />Periksa lagi
        </button>
      </div>

      <div style={grid}>
        {health.signals.map((signal) => <SignalCard key={signal.key} signal={signal} />)}
      </div>

      <div style={noteBox}>
        <p style={{ ...muted, fontSize: 12, margin: 0, lineHeight: 1.7 }}>
          <strong>Yang sehat ikut ditampilkan, dengan sengaja.</strong> Layar yang hanya menampilkan masalah tidak bisa
          dibedakan dari layar yang berhenti bekerja — &ldquo;tidak ada peringatan&rdquo; harus berarti
          &ldquo;sudah diperiksa, aman&rdquo;, bukan &ldquo;mungkin tidak ada yang memeriksa&rdquo;.
          <br /><br />
          <strong>&ldquo;Tidak dipantau&rdquo; bukan hijau.</strong> Sebuah butir yang belum ada sumber datanya
          ditandai begitu apa adanya, bukan diberi lampu hijau. Lampu hijau yang tidak memeriksa apa pun lebih buruk
          daripada tidak ada lampu sama sekali.
        </p>
      </div>
    </section>
  );
}

function SignalCard({ signal }: { signal: HealthSignal }) {
  const tone = TONE[signal.status] ?? TONE.UNMONITORED!;
  return (
    <div style={{ ...card, background: tone.background }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, color: tone.colour, fontWeight: 700, fontSize: 13 }}>
        <StatusIcon status={signal.status} />
        {tone.label}
      </div>
      <h3 style={{ margin: "10px 0 6px", fontSize: 15 }}>{signal.title}</h3>
      <p style={{ margin: 0, fontSize: 13, lineHeight: 1.6, color: "var(--color-warm-700)" }}>{signal.detail}</p>
      <dl style={metaList}>
        {signal.affectedTenants > 0 && (
          <div style={metaRow}>
            <dt style={metaLabel}>Travel terdampak</dt>
            <dd style={metaValue}>{signal.affectedTenants}</dd>
          </div>
        )}
        {signal.oldestSeen && (
          <div style={metaRow}>
            <dt style={metaLabel}>{signal.status === "OK" ? "Terakhir" : "Sejak"}</dt>
            <dd style={metaValue}>{timeOf(signal.oldestSeen)}</dd>
          </div>
        )}
        <div style={metaRow}>
          <dt style={metaLabel}>Sumber</dt>
          <dd style={{ ...metaValue, fontWeight: 400, color: "var(--color-warm-400)" }}>{signal.source}</dd>
        </div>
      </dl>
    </div>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: "4px 0 0" };
const heading: React.CSSProperties = { margin: 0, fontSize: 18 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(290px,1fr))", gap: 12 };
const card: React.CSSProperties = { border: "1px solid var(--color-cream-300)", borderRadius: 12, padding: 18 };
const metaList: React.CSSProperties = { margin: "12px 0 0", display: "grid", gap: 4 };
const metaRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 10, fontSize: 11 };
const metaLabel: React.CSSProperties = { color: "var(--color-warm-400)", margin: 0 };
const metaValue: React.CSSProperties = { margin: 0, fontWeight: 700, color: "var(--color-warm-700)", textAlign: "right" };
const noteBox: React.CSSProperties = { padding: "16px 18px", border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "var(--color-cream-100)" };
const refreshButton: React.CSSProperties = { minHeight: 40, padding: "0 16px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", color: "var(--color-emerald-900)", font: "inherit", fontWeight: 700, fontSize: 13, display: "inline-flex", alignItems: "center", gap: 6, cursor: "pointer" };
const errorBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, padding: "12px 16px", borderRadius: 8, background: "var(--color-danger-100)", color: "var(--color-danger-600)", fontSize: 13, fontWeight: 600 };
