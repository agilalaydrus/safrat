"use client";

import { useEffect, useMemo, useState } from "react";
import { IconAlertTriangle } from "@tabler/icons-react";
import type { UsageRow } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const METRIC_LABEL: Record<string, string> = {
  pilgrims: "Jamaah",
  branches: "Cabang",
  storage_bytes: "Penyimpanan",
};

function formatValue(metric: string, value: bigint): string {
  if (metric !== "storage_bytes") return new Intl.NumberFormat("id-ID").format(Number(value));
  const mb = Number(value) / 1_048_576;
  return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${mb.toFixed(1)} MB`;
}

/** Zero is a real limit — STARTER allows no branches at all — so it must never
 *  render the same as "no limit at all". */
function formatLimit(metric: string, limit?: bigint): string {
  if (limit === undefined || limit === null) return "Tanpa batas";
  return formatValue(metric, limit);
}

function ratio(value: bigint, limit?: bigint): number | undefined {
  if (limit === undefined || limit === null) return undefined;
  if (Number(limit) === 0) return Number(value) > 0 ? 1 : 0;
  return Number(value) / Number(limit);
}

export default function UsageTab() {
  const [rows, setRows] = useState<UsageRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    platformClient
      .listUsage({})
      .then((response) => setRows(response.rows))
      .catch(() => setNotice("Gagal memuat pemakaian."))
      .finally(() => setLoading(false));
  }, []);

  const atLimit = useMemo(
    () => rows.filter((r) => { const p = ratio(r.value, r.limit); return p !== undefined && p >= 1; }),
    [rows],
  );
  const nearLimit = useMemo(
    () => rows.filter((r) => { const p = ratio(r.value, r.limit); return p !== undefined && p >= 0.8 && p < 1; }),
    [rows],
  );
  const computedAt = useMemo(
    () => rows.reduce<Date | undefined>((latest, r) => {
      const at = r.computedAt?.toDate();
      return at && (!latest || at > latest) ? at : latest;
    }, undefined),
    [rows],
  );

  if (loading) return <p style={muted}>Memuat pemakaian…</p>;

  return (
    <section style={{ display: "grid", gap: 18 }}>
      <div>
        <h2 style={heading}>Pemakaian</h2>
        <p style={muted}>
          {rows.length} baris · {atLimit.length} sudah mentok · {nearLimit.length} mendekati batas
          {computedAt ? ` · dihitung ${computedAt.toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}` : ""}
        </p>
        <p style={{ ...muted, fontSize: 12, marginTop: 4 }}>
          Diambil sekali sehari, bukan dihitung saat halaman dibuka — angkanya bisa tertinggal
          beberapa jam dari kenyataan.
        </p>
      </div>

      {notice && <p role="status" style={noticeBox}>{notice}</p>}

      {atLimit.length > 0 && (
        <p style={warnBox}>
          <IconAlertTriangle size={16} />
          {atLimit.length} travel sudah mentok batasnya. Mereka tidak bisa menambah data baru —
          dan biasanya baru tahu saat penambahan ditolak, bukan sebelumnya.
        </p>
      )}

      {rows.length === 0 ? (
        <div style={emptyBox}>
          <p style={{ margin: 0, fontWeight: 700 }}>Belum ada data pemakaian</p>
          <p style={{ ...muted, marginTop: 6 }}>
            Snapshot diambil worker harian. Kalau ini kosong setelah sehari, periksa antrean worker
            di layar Kesehatan.
          </p>
        </div>
      ) : (
        <table style={table}>
          <caption style={caption}>Lintas seluruh travel · yang mendekati batas di atas</caption>
          <thead>
            <tr>{["Travel", "Paket", "Metrik", "Pemakaian", "Batas", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
          </thead>
          <tbody>
            {[...rows]
              .sort((a, b) => (ratio(b.value, b.limit) ?? -1) - (ratio(a.value, a.limit) ?? -1))
              .map((row) => {
                const pct = ratio(row.value, row.limit);
                const tone = pct === undefined ? undefined : pct >= 1 ? "danger" : pct >= 0.8 ? "warning" : "ok";
                return (
                  <tr key={`${row.operatorId}-${row.metric}`} style={tone === "danger" ? { ...tr, background: "var(--color-danger-100)" } : tone === "warning" ? { ...tr, background: "var(--color-warning-50)" } : tr}>
                    <td style={{ ...td, fontWeight: 700 }}>{row.operatorName}</td>
                    <td style={td}>{row.plan}</td>
                    <td style={td}>{METRIC_LABEL[row.metric] ?? row.metric}</td>
                    <td style={td}>{formatValue(row.metric, row.value)}</td>
                    <td style={td}>{formatLimit(row.metric, row.limit)}</td>
                    <td style={{ ...td, width: 180 }}>
                      {pct === undefined ? (
                        <span style={{ color: "var(--color-warm-400)", fontSize: 12 }}>—</span>
                      ) : (
                        <>
                          <div style={barTrack}>
                            <div style={{
                              ...barFill,
                              width: `${Math.min(100, Math.round(pct * 100))}%`,
                              background: tone === "danger" ? "var(--color-danger-600)" : tone === "warning" ? "var(--color-warning-600)" : "var(--color-emerald-800)",
                            }} />
                          </div>
                          <small style={{ color: "var(--color-warm-400)" }}>{Math.round(pct * 100)}%</small>
                        </>
                      )}
                    </td>
                  </tr>
                );
              })}
          </tbody>
        </table>
      )}

      <div style={noteBox}>
        <p style={{ margin: "0 0 6px", fontWeight: 700, fontSize: 13 }}>Catatan Metodologi</p>
        <p style={{ ...muted, fontSize: 12, margin: 0 }}>
          Jamaah dihitung tanpa yang sudah digantikan, sama seperti yang dipakai penegakan batas —
          kalau berbeda, angka di sini akan bertengkar dengan penolakan yang dialami travel.
          Cabang dihitung yang aktif saja. Penyimpanan menjumlahkan berkas storefront yang sudah
          terkonfirmasi. <strong>Panggilan API dan pesan WhatsApp belum diukur</strong> karena belum
          ada yang mencatatnya — sengaja tidak ditampilkan sebagai nol, supaya tidak terbaca sebagai
          travel yang tidak memakainya.
        </p>
      </div>
    </section>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: 0 };
const heading: React.CSSProperties = { margin: "0 0 4px", fontSize: 18 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const caption: React.CSSProperties = { captionSide: "top", textAlign: "left", padding: "0 0 8px", fontSize: 11, color: "var(--color-warm-400)", letterSpacing: "0.06em", textTransform: "uppercase" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "middle", fontSize: 13 };
const barTrack: React.CSSProperties = { height: 6, borderRadius: 999, background: "var(--color-cream-300)", overflow: "hidden", marginBottom: 4 };
const barFill: React.CSSProperties = { height: "100%", borderRadius: 999, transition: "width 700ms ease-out" };
const noticeBox: React.CSSProperties = { margin: 0, padding: "10px 14px", borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)", fontSize: 13 };
const warnBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: 0, padding: "12px 16px", background: "var(--color-warning-50)", border: "1px solid var(--color-warning-200)", borderRadius: 8, color: "var(--color-warning-700)", fontSize: 13, fontWeight: 600 };
const emptyBox: React.CSSProperties = { padding: "20px 18px", border: "1px dashed var(--color-cream-400)", borderRadius: 10, background: "#fff" };
const noteBox: React.CSSProperties = { padding: "14px 16px", border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "var(--color-cream-100)" };
