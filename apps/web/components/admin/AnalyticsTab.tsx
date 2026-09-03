"use client";

import { useEffect, useMemo, useState } from "react";
import { IconAlertTriangle, IconInfoCircle } from "@tabler/icons-react";
import type { GetPlatformAnalyticsResponse } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const rupiah = (n: bigint | number) => `Rp${Number(n).toLocaleString("id-ID")}`;
const count = (n: number) => new Intl.NumberFormat("id-ID").format(n);
const PERIODS: [number, string][] = [[30, "30 hari"], [90, "90 hari"], [365, "1 tahun"]];

export default function AnalyticsTab() {
  const [days, setDays] = useState(30);
  const [data, setData] = useState<GetPlatformAnalyticsResponse>();
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");

  useEffect(() => {
    setLoading(true);
    setFailure("");
    platformClient
      .getPlatformAnalytics({ days })
      .then(setData)
      .catch(() => setFailure("Gagal memuat analitik."))
      .finally(() => setLoading(false));
  }, [days]);

  const nrr = useMemo(() => {
    if (!data || data.mrrAtWindowStart <= 0n) return undefined;
    const start = Number(data.mrrAtWindowStart);
    const retained = start + Number(data.expansionMrrIdr) - Number(data.contractionMrrIdr) - Number(data.churnedMrrIdr);
    return (retained / start) * 100;
  }, [data]);

  if (loading) return <p style={muted}>Memuat analitik…</p>;
  if (failure) return <p style={errorBox}><IconAlertTriangle size={16} />{failure}</p>;
  if (!data) return null;

  const trialRate = data.trialsStarted > 0 ? (data.trialsConverted / data.trialsStarted) * 100 : undefined;
  const net = Number(data.newMrrIdr) + Number(data.expansionMrrIdr) - Number(data.contractionMrrIdr) - Number(data.churnedMrrIdr);

  return (
    <section style={{ display: "grid", gap: 18 }}>
      <div style={{ display: "flex", justifyContent: "space-between", gap: 16, flexWrap: "wrap", alignItems: "flex-start" }}>
        <div>
          <h2 style={heading}>Analitik</h2>
          <p style={muted}>Pendapatan berulang dari langganan. Komisi marketplace tidak termasuk — lihat catatan di bawah.</p>
        </div>
        <div style={periodBar} role="group" aria-label="Rentang waktu">
          {PERIODS.map(([value, label]) => (
            <button key={value} type="button" onClick={() => setDays(value)} aria-pressed={days === value}
              style={days === value ? periodActive : periodInactive}>{label}</button>
          ))}
        </div>
      </div>

      <div style={statGrid}>
        {[
          { label: "MRR", value: rupiah(data.mrrIdr), hint: `${count(data.payingTenants)} travel membayar` },
          { label: "Pergerakan bersih", value: `${net >= 0 ? "+" : "−"}${rupiah(Math.abs(net))}`, hint: `dalam ${data.days} hari`, tone: net >= 0 ? "var(--color-emerald-900)" : "var(--color-danger-600)" },
          { label: "NRR", value: nrr === undefined ? "—" : `${nrr.toFixed(1)}%`, hint: nrr === undefined ? "belum ada dasar hitungan" : nrr < 100 ? "ekspansi belum menutup churn" : "ekspansi menutup churn" },
          { label: "Konversi trial", value: trialRate === undefined ? "—" : `${trialRate.toFixed(0)}%`, hint: `${count(data.trialsConverted)} dari ${count(data.trialsStarted)} yang mulai` },
        ].map((card) => (
          <div key={card.label} style={statCard}>
            <p style={statLabel}>{card.label}</p>
            <p style={{ ...statValue, color: card.tone ?? "var(--color-emerald-900)" }}>{card.value}</p>
            <p style={statHint}>{card.hint}</p>
          </div>
        ))}
      </div>

      <div style={noteBox}>
        <h3 style={{ margin: "0 0 12px", fontSize: 14 }}>Pergerakan MRR dalam {data.days} hari</h3>
        <div style={{ display: "grid", gap: 10 }}>
          {[
            ["Awal periode", data.mrrAtWindowStart, "var(--color-warm-400)"],
            ["Travel baru", data.newMrrIdr, "var(--color-emerald-800)"],
            ["Naik paket", data.expansionMrrIdr, "var(--color-emerald-600)"],
            ["Turun paket", data.contractionMrrIdr, "var(--color-warning-600)"],
            ["Berhenti", data.churnedMrrIdr, "var(--color-danger-600)"],
            ["Sekarang", data.mrrIdr, "var(--color-emerald-900)"],
          ].map(([label, value, colour]) => {
            const amount = Number(value);
            const scale = Math.max(Number(data.mrrIdr), Number(data.mrrAtWindowStart), 1);
            return (
              <div key={String(label)}>
                <div style={rowHead}>
                  <span>{String(label)}</span>
                  <span style={{ fontWeight: 700 }}>{rupiah(amount)}</span>
                </div>
                <div style={track}>
                  <div style={{ width: `${Math.min((amount / scale) * 100, 100)}%`, height: "100%", background: String(colour), borderRadius: 5 }} />
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div>
        <h3 style={{ margin: "0 0 4px", fontSize: 16 }}>Per paket</h3>
        <p style={muted}>Trial dan yang masa aktifnya habis dipisah, karena keduanya belum membayar.</p>
        <table style={{ ...table, marginTop: 12 }}>
          <thead>
            <tr>{["Paket", "Harga/bulan", "Membayar", "MRR", "Trial", "Habis masa"].map((head) => <th key={head} style={th}>{head}</th>)}</tr>
          </thead>
          <tbody>
            {data.byPlan.map((plan) => (
              <tr key={plan.plan} style={tr}>
                <td style={{ ...td, fontWeight: 700 }}>{plan.plan}</td>
                <td style={td}>{rupiah(plan.monthlyIdr)}</td>
                <td style={td}>{count(plan.payingTenants)}</td>
                <td style={{ ...td, fontWeight: 700 }}>{rupiah(plan.mrrIdr)}</td>
                <td style={td}>{count(plan.trialTenants)}</td>
                <td style={td}>{plan.lapsedTenants > 0
                  ? <span style={{ color: "var(--color-warning-700)", fontWeight: 700 }}>{count(plan.lapsedTenants)}</span>
                  : "0"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div style={statGrid}>
        {[
          { label: "Sedang trial", value: count(data.trialingTenants), hint: "belum menyumbang MRR" },
          { label: "Ditangguhkan", value: count(data.suspendedTenants), hint: "dihentikan dengan sengaja" },
          { label: "Habis masa", value: count(data.lapsedTenants), hint: "tidak dibatalkan, tapi tidak bisa masuk" },
          { label: "Berhenti", value: count(data.churnedTenants), hint: `dalam ${data.days} hari` },
        ].map((card) => (
          <div key={card.label} style={statCard}>
            <p style={statLabel}>{card.label}</p>
            <p style={{ ...statValue, fontSize: 20 }}>{card.value}</p>
            <p style={statHint}>{card.hint}</p>
          </div>
        ))}
      </div>

      <section style={methodology}>
        <h3 style={{ margin: "0 0 12px", fontSize: 14, display: "flex", alignItems: "center", gap: 8 }}>
          <IconInfoCircle size={16} />Cara angka ini dihitung
        </h3>
        <ul style={methodList}>
          <li>
            <strong>Komisi marketplace bukan MRR.</strong> Ia pendapatan lain dan sengaja tidak ada di sini.
            Mencampurnya membuat pertumbuhan terlihat lebih cepat daripada sebenarnya.
          </li>
          <li>
            MRR hanya menghitung travel yang <strong>benar-benar bisa memakai produknya</strong>: bukan trial, tidak
            dibatalkan, tidak ditangguhkan, dan masa bayarnya belum habis. Definisinya sama persis dengan pemeriksaan
            akses yang menentukan mereka bisa masuk atau tidak — MRR yang menghitung orang yang terkunci adalah angka
            yang menyanjung kita tepat ketika seharusnya tidak.
          </li>
          <li>
            <strong>NRR di bawah 100% berarti ekspansi tidak menutup churn.</strong> Ia dihitung dari MRR awal periode
            yang <em>direkonstruksi</em> dari pergerakan, bukan dari potret bulanan — kami belum menyimpannya.
            Akibatnya: travel yang naik paket lalu berhenti di periode yang sama muncul di churn, bukan di ekspansi.
          </li>
          <li>
            Konversi trial diukur pada <strong>rombongan yang mulai di periode ini</strong>. Trial yang masih berjalan
            ikut di penyebut, jadi angkanya baru mengendap setelah periodenya lewat — membagi konversi bulan ini dengan
            pendaftar bulan ini membandingkan dua kelompok orang yang berbeda.
          </li>
          <li>
            Tidak ada skor risiko churn di layar ini. Kalau nanti ada, ia <strong>heuristik internal</strong> — penanda
            prioritas, bukan vonis, dan tidak boleh dipakai untuk memutus hubungan dengan pelanggan.
          </li>
        </ul>
      </section>
    </section>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: "4px 0 0" };
const heading: React.CSSProperties = { margin: 0, fontSize: 18 };
const statGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(190px,1fr))", gap: 12 };
const statCard: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 10, padding: "16px 18px" };
const statLabel: React.CSSProperties = { margin: "0 0 6px", fontSize: 12, color: "var(--color-warm-500)", fontWeight: 600 };
const statValue: React.CSSProperties = { margin: 0, fontSize: 22, fontWeight: 700 };
const statHint: React.CSSProperties = { margin: "4px 0 0", fontSize: 11, color: "var(--color-warm-400)" };
const noteBox: React.CSSProperties = { padding: "16px 18px", border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "#fff" };
const rowHead: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 12, fontSize: 13, marginBottom: 5 };
const track: React.CSSProperties = { height: 10, background: "var(--color-cream-300)", borderRadius: 5, overflow: "hidden" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff", fontSize: 13 };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)" };
const methodology: React.CSSProperties = { padding: "18px 20px", border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "var(--color-cream-100)" };
const methodList: React.CSSProperties = { margin: 0, paddingLeft: 20, display: "grid", gap: 10, fontSize: 13, lineHeight: 1.65, color: "var(--color-warm-700)" };
const periodBar: React.CSSProperties = { display: "inline-flex", gap: 4, padding: 4, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 10 };
const periodBase: React.CSSProperties = { minHeight: 38, padding: "0 14px", border: 0, borderRadius: 7, background: "transparent", font: "inherit", fontSize: 13, fontWeight: 700, color: "var(--color-warm-500)", cursor: "pointer" };
const periodActive: React.CSSProperties = { ...periodBase, background: "#fff", color: "var(--color-emerald-900)", boxShadow: "0 1px 2px rgba(0,0,0,.06)" };
const periodInactive: React.CSSProperties = periodBase;
const errorBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, padding: "12px 16px", borderRadius: 8, background: "var(--color-danger-100)", color: "var(--color-danger-600)", fontSize: 13, fontWeight: 600 };
