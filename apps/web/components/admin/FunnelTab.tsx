"use client";

import { useEffect, useMemo, useState } from "react";
import { IconAlertTriangle, IconExternalLink } from "@tabler/icons-react";
import type { GetPlatformFunnelResponse, StorefrontFunnelRow } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { ActionCenter, type ActionCenterItem } from "@/components/ui/ActionCenter";
import { buildTenantLink } from "@/lib/tenant-link";
import { platformClient } from "@/lib/rpc";

const PERIODS: [number, string][] = [[7, "7 hari"], [30, "30 hari"], [90, "90 hari"]];
const number = (n: number) => new Intl.NumberFormat("id-ID").format(n);
const percent = (n: number) => `${(n * 100).toFixed(1)}%`;

// TawafiqHub's own funnel, in the order it is walked. DAFTAR is our sign-up
// page — deliberately its own step and not KATALOG, which means something else
// on a storefront.
const PLATFORM_STEPS: [string, string][] = [
  ["LANDING", "Membuka tawafiqhub.id"],
  ["ARTIKEL", "Membaca artikel"],
  ["DAFTAR", "Membuka halaman daftar"],
];

export default function FunnelTab() {
  const [days, setDays] = useState(30);
  const [report, setReport] = useState<GetPlatformFunnelResponse>();
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    setLoading(true);
    setNotice("");
    platformClient
      .getPlatformFunnel({ days })
      .then(setReport)
      .catch(() => setNotice("Gagal memuat corong."))
      .finally(() => setLoading(false));
  }, [days]);

  const platformSteps = useMemo(() => {
    const found = new Map(report?.platformSteps.map((step) => [step.step, step.visitors]) ?? []);
    return PLATFORM_STEPS.map(([id, label]) => ({ id, label, visitors: found.get(id) ?? 0 }));
  }, [report]);

  // Storefronts nobody opened at all. Capped at eight: the Action Centre is a
  // list of things to do this week, and a list of forty is a list of none.
  const silentActions: ActionCenterItem[] | undefined = useMemo(() => {
    if (!report) return undefined;
    return report.silentStorefronts.slice(0, 8).map((row) => ({
      id: row.operatorId,
      title: `${row.operatorName} — storefront tidak pernah dibuka`,
      description:
        `Tidak ada satu pun pengunjung dalam ${days} hari terakhir. Travel yang membayar untuk sesuatu ` +
        `yang tidak dipakai adalah travel yang akan berhenti berlangganan. Biasanya tautannya belum ` +
        `pernah disebar, bukan storefront-nya rusak.`,
      financialImpact: "Risiko berhenti berlangganan",
      actionHref: buildTenantLink(row.slug, "/") || "#",
      actionLabel: "Buka storefront",
      tone: "warning" as const,
    }));
  }, [report, days]);

  const entry = platformSteps[0]?.visitors ?? 0;
  const newTenants = report?.newTenants ?? 0;
  const aggregateRate = report && report.totalVisitors > 0 ? report.totalRegistrations / report.totalVisitors : 0;

  if (loading) return <p style={muted}>Memuat corong…</p>;

  return (
    <section style={{ display: "grid", gap: 18 }}>
      <div style={{ display: "flex", justifyContent: "space-between", gap: 16, flexWrap: "wrap", alignItems: "flex-start" }}>
        <div>
          <h2 style={heading}>Corong</h2>
          <p style={muted}>Lintas seluruh travel, dan corong TawafiqHub sendiri di sebelahnya.</p>
        </div>
        <div style={periodBar} role="group" aria-label="Rentang waktu">
          {PERIODS.map(([value, label]) => (
            <button key={value} type="button" onClick={() => setDays(value)} aria-pressed={days === value} style={days === value ? periodActive : periodInactive}>
              {label}
            </button>
          ))}
        </div>
      </div>

      {notice && <p role="status" style={warnBox}><IconAlertTriangle size={16} />{notice}</p>}

      {report && (
        <>
          <div style={statGrid}>
            {[
              { label: "Pengunjung tawafiqhub.id", value: number(entry), hint: "situs kita sendiri" },
              { label: "Tenant baru", value: number(newTenants), hint: `dalam ${days} hari` },
              { label: "Pengunjung seluruh storefront", value: number(report.totalVisitors), hint: "angka untuk dikutip saat menjual" },
              { label: "Pendaftar seluruh storefront", value: number(report.totalRegistrations), hint: `konversi ${percent(aggregateRate)}` },
            ].map((card) => (
              <div key={card.label} style={statCard}>
                <p style={statLabel}>{card.label}</p>
                <p style={statValue}>{card.value}</p>
                <p style={statHint}>{card.hint}</p>
              </div>
            ))}
          </div>

          <div style={noteBox}>
            <h3 style={{ margin: "0 0 10px", fontSize: 14 }}>Corong penjualan TawafiqHub</h3>
            <div style={{ display: "grid", gap: 10 }}>
              {platformSteps.map((step, index) => {
                const share = entry > 0 ? (step.visitors / entry) * 100 : 0;
                const shade = ["var(--color-gold-400)", "var(--color-gold-500)", "var(--color-emerald-800)"][index];
                return (
                  <div key={step.id}>
                    <div style={rowHead}>
                      <span>{step.label}</span>
                      <span style={{ fontWeight: 700 }}>{number(step.visitors)} <span style={{ color: "var(--color-warm-400)", fontWeight: 500 }}>· {share.toFixed(1)}%</span></span>
                    </div>
                    <div style={track}><div style={{ width: `${Math.min(share, 100)}%`, height: "100%", background: shade, borderRadius: 5 }} /></div>
                  </div>
                );
              })}
              {/* Counted from operators, not from a page view: somebody who
                  opened the sign-up form and never became a tenant has not
                  converted, and treating the page as the outcome would make
                  this funnel flatter than it is. */}
              <div>
                <div style={rowHead}>
                  <span>Menjadi tenant</span>
                  <span style={{ fontWeight: 700 }}>{number(newTenants)} <span style={{ color: "var(--color-warm-400)", fontWeight: 500 }}>· {entry > 0 ? ((newTenants / entry) * 100).toFixed(1) : "0.0"}%</span></span>
                </div>
                <div style={track}><div style={{ width: `${entry > 0 ? Math.min((newTenants / entry) * 100, 100) : 0}%`, height: "100%", background: "var(--color-emerald-950)", borderRadius: 5 }} /></div>
              </div>
            </div>
            <p style={{ ...muted, fontSize: 12, marginTop: 12 }}>
              Langkah terakhir dihitung dari tabel travel, bukan dari kunjungan halaman: pendaftaran yang tidak pernah
              jadi tenant bukan konversi.
            </p>
          </div>

          <ActionCenter
            items={silentActions}
            title="Pusat Tindakan — Storefront Sepi"
            subtitle={`Travel yang storefront-nya tidak dibuka siapa pun dalam ${days} hari terakhir`}
            cleanTitle="Semua storefront ada pengunjungnya"
            cleanDescription="Tidak ada travel yang membayar untuk halaman yang tidak pernah dibuka."
          />

          <div>
            <h3 style={{ margin: "0 0 4px", fontSize: 16 }}>Papan Peringkat Storefront</h3>
            <p style={muted}>
              Urut konversi, terbaik di atas. <strong>Yang di bawah adalah daftar kerja, bukan papan malu</strong> —
              storefront ramai tanpa pendaftar biasanya salah pasang harga atau formulirnya rusak, dan itu bisa dibantu.
            </p>
            {report.storefronts.length === 0 ? (
              <div style={{ ...emptyBox, marginTop: 12 }}>
                <p style={{ margin: 0, fontWeight: 700 }}>Belum ada storefront yang cukup ramai untuk diperingkat</p>
                <p style={{ ...muted, marginTop: 6 }}>
                  Butuh minimal {report.rankingFloor} pengunjung dalam rentang ini sebelum konversinya berarti.
                </p>
              </div>
            ) : (
              <table style={{ ...table, marginTop: 12 }}>
                <thead>
                  <tr>{["Travel", "Pengunjung", "Pendaftar", "Konversi", ""].map((head) => <th key={head} style={th}>{head}</th>)}</tr>
                </thead>
                <tbody>
                  {report.storefronts.map((row, index) => (
                    <StorefrontRow key={row.operatorId} row={row} rank={index + 1} total={report.storefronts.length} />
                  ))}
                </tbody>
              </table>
            )}
          </div>

          {report.tooFewVisitors.length > 0 && (
            <div>
              <h3 style={{ margin: "0 0 4px", fontSize: 16 }}>Belum cukup ramai untuk diperingkat</h3>
              <p style={muted}>
                Di bawah {report.rankingFloor} pengunjung. Tiga pengunjung dengan satu pendaftar berarti konversi 33% dan
                tidak berarti apa-apa — ditampilkan terpisah supaya tidak ikut menaiki papan peringkat, dan tidak
                disembunyikan supaya tidak hilang.
              </p>
              <table style={{ ...table, marginTop: 12 }}>
                <thead>
                  <tr>{["Travel", "Pengunjung", "Pendaftar", "", ""].map((head, index) => <th key={index} style={th}>{head}</th>)}</tr>
                </thead>
                <tbody>
                  {report.tooFewVisitors.map((row) => (
                    <tr key={row.operatorId} style={tr}>
                      <td style={{ ...td, fontWeight: 700 }}>{row.operatorName}</td>
                      <td style={td}>{number(row.visitors)}</td>
                      <td style={td}>{number(row.registrations)}</td>
                      <td style={td} />
                      <td style={td}><StorefrontLink slug={row.slug} /></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div style={noteBox}>
            <h3 style={{ margin: "0 0 8px", fontSize: 14 }}>Cara angka ini dihitung</h3>
            <p style={{ ...muted, fontSize: 12, lineHeight: 1.7 }}>
              Pengunjung dihitung sekali per hari dan berganti tiap tengah malam, jadi orang yang sama pada dua hari
              terhitung dua kali dan atribusi lintas hari tidak akurat — konsekuensi langsung dari tidak memakai cookie.
              Penanda dengan lebih dari 60 kejadian sehari dibuang seluruhnya sebagai mesin perayap. Hari memakai Waktu
              Indonesia Barat. Angka pendaftar diambil dari baris pendaftaran, bukan dari kejadian corong, supaya tidak
              hilang saat penanda pengunjung berganti.
            </p>
          </div>
        </>
      )}
    </section>
  );
}

function StorefrontRow({ row, rank, total }: { row: StorefrontFunnelRow; rank: number; total: number }) {
  // Only the bottom of a board long enough to have a bottom. Marking the last
  // of two storefronts as needing help says nothing about that storefront.
  const needsHelp = total >= 5 && rank > total - 3 && row.conversion < 0.01;
  return (
    <tr style={needsHelp ? { ...tr, background: "var(--color-warning-50)" } : tr}>
      <td style={{ ...td, fontWeight: 700 }}>
        {row.operatorName}
        {needsHelp && <span style={helpTag}>perlu dibantu</span>}
      </td>
      <td style={td}>{number(row.visitors)}</td>
      <td style={td}>{number(row.registrations)}</td>
      <td style={td}>
        <div style={barTrack}>
          <div style={{ ...barFill, width: `${Math.min(row.conversion * 100 * 5, 100)}%`, background: row.conversion >= 0.02 ? "var(--color-emerald-700)" : "var(--color-warning-600)" }} />
        </div>
        <span style={{ fontWeight: 700 }}>{percent(row.conversion)}</span>
      </td>
      <td style={td}><StorefrontLink slug={row.slug} /></td>
    </tr>
  );
}

function StorefrontLink({ slug }: { slug: string }) {
  const href = buildTenantLink(slug, "/");
  if (!href) return null;
  return (
    <a href={href} target="_blank" rel="noreferrer" style={link}>
      Buka <IconExternalLink size={13} />
    </a>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: 0 };
const heading: React.CSSProperties = { margin: "0 0 4px", fontSize: 18 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "middle", fontSize: 13 };
const barTrack: React.CSSProperties = { height: 6, borderRadius: 999, background: "var(--color-cream-300)", overflow: "hidden", marginBottom: 4, maxWidth: 140 };
const barFill: React.CSSProperties = { height: "100%", borderRadius: 999, transition: "width 700ms ease-out" };
const warnBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: 0, padding: "12px 16px", background: "var(--color-warning-50)", border: "1px solid var(--color-warning-200)", borderRadius: 8, color: "var(--color-warning-700)", fontSize: 13, fontWeight: 600 };
const emptyBox: React.CSSProperties = { padding: "20px 18px", border: "1px dashed var(--color-cream-400)", borderRadius: 10, background: "#fff" };
const noteBox: React.CSSProperties = { padding: "16px 18px", border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "var(--color-cream-100)" };
const statGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(200px,1fr))", gap: 12 };
const statCard: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 10, padding: "16px 18px" };
const statLabel: React.CSSProperties = { margin: "0 0 6px", fontSize: 12, color: "var(--color-warm-500)", fontWeight: 600 };
const statValue: React.CSSProperties = { margin: 0, fontSize: 24, fontWeight: 700, color: "var(--color-emerald-900)" };
const statHint: React.CSSProperties = { margin: "4px 0 0", fontSize: 11, color: "var(--color-warm-400)" };
const rowHead: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 12, fontSize: 13, marginBottom: 5 };
const track: React.CSSProperties = { height: 10, background: "var(--color-cream-300)", borderRadius: 5, overflow: "hidden" };
const periodBar: React.CSSProperties = { display: "inline-flex", gap: 4, padding: 4, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 10 };
const periodBase: React.CSSProperties = { minHeight: 38, padding: "0 14px", border: 0, borderRadius: 7, background: "transparent", font: "inherit", fontSize: 13, fontWeight: 700, color: "var(--color-warm-500)", cursor: "pointer" };
const periodActive: React.CSSProperties = { ...periodBase, background: "#fff", color: "var(--color-emerald-900)", boxShadow: "0 1px 2px rgba(0,0,0,.06)" };
const periodInactive: React.CSSProperties = periodBase;
const helpTag: React.CSSProperties = { marginLeft: 8, padding: "2px 8px", borderRadius: 99, background: "var(--color-warning-200)", color: "var(--color-warning-700)", fontSize: 10, fontWeight: 800, letterSpacing: ".04em" };
const link: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-emerald-800)", fontWeight: 700, fontSize: 12, textDecoration: "none" };
