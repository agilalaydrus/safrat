"use client";

import { useCallback, useEffect, useState } from "react";
import { IconEye, IconCheck, IconX, IconAlertTriangle } from "@tabler/icons-react";
import { KycRecordSummary } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const day = (d?: Date) => d?.toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" }) ?? "—";

const STATUS: Record<string, { label: string; tone: string }> = {
  PENDING_REVIEW: { label: "Menunggu Ditinjau", tone: "#b45309" },
  VERIFIED: { label: "Terverifikasi", tone: "var(--color-emerald-800)" },
  REJECTED: { label: "Ditolak", tone: "var(--color-danger-600)" },
};

type Detail = {
  summary: KycRecordSummary;
  nik: string;
  npwp: string;
  address: string;
  placeOfBirth: string;
};

export default function IdentityTab() {
  const [records, setRecords] = useState<KycRecordSummary[]>([]);
  const [status, setStatus] = useState("PENDING_REVIEW");
  const [loading, setLoading] = useState(true);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    platformClient.listKycRecords({ status, limit: 50 })
      .then((r) => setRecords(r.records))
      .catch(() => setError("Gagal memuat data identitas."))
      .finally(() => setLoading(false));
  }, [status]);
  useEffect(() => { load(); }, [load]);

  const open = async (record: KycRecordSummary) => {
    setError("");
    try {
      const result = await platformClient.getKycRecord({
        subjectType: record.subjectType, subjectId: record.subjectId,
      });
      if (!result.summary) return;
      setDetail({
        summary: result.summary, nik: result.nik, npwp: result.npwp,
        address: result.address, placeOfBirth: result.placeOfBirth,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal membuka data identitas.");
    }
  };

  const decide = async (recordId: string, next: "VERIFIED" | "REJECTED") => {
    let reason = "";
    if (next === "REJECTED") {
      // A rejection nobody can act on gets resubmitted unchanged, so the server
      // requires a reason and so does this.
      reason = window.prompt("Alasan penolakan (akan dibaca oleh yang bersangkutan):")?.trim() ?? "";
      if (!reason) return;
    }
    setError("");
    try {
      await platformClient.setKycStatus({ recordId, status: next, reason });
      setNotice(next === "VERIFIED" ? "Identitas diverifikasi." : "Identitas ditolak.");
      setDetail(null);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengubah status.");
    }
  };

  return (
    <section style={{ display: "grid", gap: 14 }}>
      <p style={muted}>
        Nomor identitas disimpan terenkripsi dan <strong>tidak ditampilkan di daftar ini</strong> —
        satu tangkapan layar dari daftar yang memuatnya akan membocorkan semua orang di dalamnya sekaligus.
        Membuka satu identitas selalu tercatat, lengkap dengan siapa yang membukanya.
      </p>

      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        {([["PENDING_REVIEW", "Menunggu"], ["VERIFIED", "Terverifikasi"], ["REJECTED", "Ditolak"], ["", "Semua"]] as const).map(([value, label]) => (
          <button key={value} onClick={() => setStatus(value)} style={status === value ? pillActive : pill}>{label}</button>
        ))}
      </div>

      {notice && <p style={{ color: "var(--color-emerald-800)", margin: 0 }}>{notice}</p>}
      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      {loading ? <p style={muted}>Memuat...</p> : records.length === 0 ? (
        <p style={muted}>Tidak ada data identitas pada filter ini.</p>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Nama", "Travel", "Jenis", "Status", "Diajukan", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {records.map((record) => {
                const tone = STATUS[record.status] ?? { label: record.status, tone: "var(--color-warm-700)" };
                return (
                  <tr key={record.id} style={tr}>
                    <td style={td}><strong>{record.fullName || "(tanpa nama)"}</strong></td>
                    <td style={td}>{record.operatorName}</td>
                    <td style={td}>{record.subjectType === "AGENT" ? "Agen" : "Jamaah"}
                      <small style={{ display: "block", color: "var(--color-warm-400)" }}>
                        {record.source === "SELF" ? "diajukan sendiri" : "diinput staf"}
                      </small>
                    </td>
                    <td style={{ ...td, color: tone.tone, fontWeight: 600 }}>
                      {tone.label}
                      {record.rejectionReason && (
                        <small style={{ display: "block", fontWeight: 400, color: "var(--color-warm-500)", whiteSpace: "normal", maxWidth: 220 }}>
                          {record.rejectionReason}
                        </small>
                      )}
                    </td>
                    <td style={td}>{day(record.createdAt?.toDate())}</td>
                    <td style={td}>
                      <button style={ghost} onClick={() => open(record)}><IconEye size={14} />Buka</button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {detail && (
        <div style={backdrop} role="dialog" aria-modal="true" aria-label="Data identitas">
          <div style={panel}>
            <header style={head}>
              <strong style={{ fontSize: 17 }}>{detail.summary.fullName}</strong>
              <button onClick={() => setDetail(null)} style={iconButton} aria-label="Tutup"><IconX size={18} /></button>
            </header>

            <p style={warning}>
              <IconAlertTriangle size={16} />
              Pembukaan data ini sudah dicatat atas nama akun Anda.
            </p>

            <dl style={rows}>
              <Row label="NIK" value={detail.nik || "—"} />
              <Row label="NPWP" value={detail.npwp || "—"} />
              <Row label="Alamat" value={detail.address || "—"} />
              <Row label="Tempat Lahir" value={detail.placeOfBirth || "—"} />
              <Row label="Travel" value={detail.summary.operatorName} />
              <Row label="Jenis" value={detail.summary.subjectType === "AGENT" ? "Agen" : "Jamaah"} />
              <Row label="Akun tertaut" value={detail.summary.userId || "belum tertaut"} />
            </dl>

            {detail.summary.status === "PENDING_REVIEW" && (
              <footer style={foot}>
                <button style={danger} onClick={() => decide(detail.summary.id, "REJECTED")}><IconX size={16} />Tolak</button>
                <button style={accept} onClick={() => decide(detail.summary.id, "VERIFIED")}><IconCheck size={16} />Verifikasi</button>
              </footer>
            )}
          </div>
        </div>
      )}
    </section>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", gap: 16, alignItems: "baseline" }}>
      <dt style={{ color: "var(--color-warm-500)", fontSize: 13 }}>{label}</dt>
      <dd style={{ margin: 0, fontSize: 13, fontWeight: 600, textAlign: "right", overflowWrap: "anywhere" }}>{value}</dd>
    </div>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 13, margin: 0 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
const pillBase: React.CSSProperties = { minHeight: 36, borderRadius: 999, padding: "0 14px", fontSize: 13, fontWeight: 600 };
const pill: React.CSSProperties = { ...pillBase, border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-700)" };
const pillActive: React.CSSProperties = { ...pillBase, border: 0, background: "var(--color-emerald-800)", color: "#fff" };
const ghost: React.CSSProperties = { minHeight: 36, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "transparent", color: "var(--color-warm-700)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12 };
const backdrop: React.CSSProperties = { position: "fixed", inset: 0, background: "rgba(20,24,20,.45)", display: "grid", placeItems: "center", padding: 16, zIndex: 50 };
const panel: React.CSSProperties = { width: "min(520px,100%)", maxHeight: "90vh", overflowY: "auto", background: "#fff", borderRadius: 14, padding: 22, display: "grid", gap: 14 };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center" };
const iconButton: React.CSSProperties = { border: 0, background: "transparent", cursor: "pointer", color: "var(--color-warm-500)" };
const warning: React.CSSProperties = { display: "flex", gap: 8, alignItems: "center", margin: 0, padding: 10, background: "var(--color-gold-50)", borderRadius: 8, color: "#b45309", fontSize: 13 };
const rows: React.CSSProperties = { display: "grid", gap: 10, margin: 0, padding: 16, background: "var(--color-cream-100)", borderRadius: 10 };
const foot: React.CSSProperties = { display: "flex", justifyContent: "flex-end", gap: 10 };
const danger: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-danger-600)", borderRadius: 8, padding: "0 16px", background: "transparent", color: "var(--color-danger-600)", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 7 };
const accept: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 16px", background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 7 };
