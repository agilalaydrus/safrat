"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { IconAlertTriangle, IconArrowLeft } from "@tabler/icons-react";
import { CancellationPreview } from "@hajj-saas/proto-gen/hajj/v1/cancellation_pb";
import { cancellationClient } from "@/lib/rpc";
import { RoleGate } from "@/components/auth/RoleGate";

function formatIDR(value: bigint | number): string {
  return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(value));
}

export default function PilgrimCancelFlow({ pilgrimId }: { pilgrimId: string }) {
  const router = useRouter();
  const [preview, setPreview] = useState<CancellationPreview>();
  const [loading, setLoading] = useState(true);
  const [reason, setReason] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    cancellationClient.previewCancellation({ pilgrimId }).then(setPreview).catch((error) => setNotice(error instanceof Error ? error.message : "Gagal memuat pratinjau pembatalan.")).finally(() => setLoading(false));
  }, [pilgrimId]);

  const confirm = async () => {
    if (!reason.trim()) { setNotice("Alasan pembatalan wajib diisi."); return; }
    setConfirming(true);
    setNotice("");
    try {
      await cancellationClient.confirmCancellation({ pilgrimId, reason: reason.trim() });
      router.push("/dashboard/pilgrims");
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Gagal membatalkan jamaah.");
      setConfirming(false);
    }
  };

  return <main style={page}>
    <Link href={`/dashboard/pilgrims/${pilgrimId}`} style={{ color: "var(--color-gold-800)", display: "inline-flex", alignItems: "center", gap: 6 }}><IconArrowLeft size={16} />Kembali ke profil jamaah</Link>
    <header style={header}>
      <p style={eyebrow}>JAMAAH</p>
      <h1 style={title}>Batalkan Jamaah</h1>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
    {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat pratinjau...</p> : preview && <section style={card}>
      <h2 style={{ margin: 0 }}>{preview.pilgrimName}</h2>
      <div style={grid}>
        <div style={stat}><span style={statLabel}>Hari Sebelum Keberangkatan</span><strong style={statValue}>{preview.daysBefore} hari</strong></div>
        <div style={stat}><span style={statLabel}>Tingkatan Kebijakan</span><strong style={statValue}>{preview.policyName || "Tidak ada (0%)"}</strong></div>
        <div style={stat}><span style={statLabel}>Total Dibayar</span><strong style={statValue}>{formatIDR(preview.totalPaidIdr)}</strong></div>
        <div style={stat}><span style={statLabel}>Persentase Refund</span><strong style={{ ...statValue, color: "var(--color-gold-800)" }}>{preview.refundPct}%</strong></div>
      </div>
      <div style={refundBanner}>
        <p style={{ margin: 0, fontSize: 13, color: "var(--color-warm-500)" }}>Jamaah akan dikenakan refund</p>
        <p style={{ margin: "4px 0 0", fontSize: 28, fontWeight: 700, color: "var(--color-emerald-900)" }}>{formatIDR(preview.refundAmountIdr)}</p>
        <p style={{ margin: "4px 0 0", fontSize: 13, color: "var(--color-warm-400)" }}>{preview.refundPct}% dari {formatIDR(preview.totalPaidIdr)}</p>
      </div>

      <RoleGate require={["owner", "admin"]} fallback={<p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Hanya pemilik atau admin yang dapat melakukan pembatalan.</p>}>
        <label style={field}>Alasan Pembatalan (wajib)
          <textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={3} placeholder="Contoh: Jamaah sakit dan tidak dapat berangkat" style={{ ...input, minHeight: 80, resize: "vertical" }} />
        </label>
        <div style={warningBanner}>
          <IconAlertTriangle size={18} style={{ flexShrink: 0, color: "var(--color-danger-600)" }} />
          <p style={{ margin: 0, fontSize: 13, color: "var(--color-danger-600)" }}>Pembatalan bersifat permanen. Data jamaah akan dikunci dan tidak dapat diubah kembali.</p>
        </div>
        <button disabled={confirming || !reason.trim()} onClick={confirm} style={dangerBtn}>{confirming ? "Memproses..." : "Konfirmasi Pembatalan"}</button>
      </RoleGate>
    </section>}
  </main>;
}

const page: React.CSSProperties = { maxWidth: 700, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { marginTop: 20 };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,44px)", fontWeight: 500, margin: 0 };
const card: React.CSSProperties = { display: "grid", gap: 16, background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 24, marginTop: 20 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))", gap: 12 };
const stat: React.CSSProperties = { border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "var(--color-cream-100)", padding: "12px 16px", display: "grid", gap: 4 };
const statLabel: React.CSSProperties = { fontSize: 11, color: "var(--color-warm-500)", textTransform: "uppercase", letterSpacing: ".05em" };
const statValue: React.CSSProperties = { fontSize: 18, fontWeight: 700 };
const refundBanner: React.CSSProperties = { textAlign: "center", padding: "20px", background: "var(--color-emerald-50)", borderRadius: 10 };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const warningBanner: React.CSSProperties = { display: "flex", gap: 10, alignItems: "flex-start", background: "var(--color-danger-100)", border: "1px solid var(--color-danger-600)", borderRadius: 8, padding: "12px 16px" };
const dangerBtn: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-danger-600)", color: "white", fontWeight: 700, padding: "0 18px", justifySelf: "start" };
