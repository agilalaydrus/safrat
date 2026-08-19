"use client";
import { useEffect, useState } from "react";
import { IconMapPin, IconMapPinExclamation } from "@tabler/icons-react";
import { LostReport } from "@hajj-saas/proto-gen/hajj/v1/lost_report_pb";
import { lostReportClient } from "@/lib/rpc";

export default function LostReportsDashboard() {
  const [reports, setReports] = useState<LostReport[]>([]);
  const [notice, setNotice] = useState("");
  const [resolvingId, setResolvingId] = useState("");

  const refresh = () => lostReportClient.listActiveLostReports({}).then((r) => setReports(r.reports)).catch(() => setNotice("Gagal memuat laporan jamaah tersesat."));

  useEffect(() => {
    refresh();
    const interval = window.setInterval(refresh, 10000);
    return () => window.clearInterval(interval);
  }, []);

  const resolve = async (id: string) => {
    setResolvingId(id);
    try { await lostReportClient.resolveLostReport({ id }); void refresh(); } catch { setNotice("Gagal menyelesaikan laporan."); } finally { setResolvingId(""); }
  };

  const active = reports.filter((r) => r.status !== "RESOLVED");

  return <main style={page}>
    <header>
      <p style={eyebrow}>OPERASIONAL / JAMAAH TERSESAT</p>
      <h1 style={title}>Jamaah Terpisah</h1>
    </header>
    <div className="gold-divider" />
    {notice && <p style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
    {!active.length ? (
      <section style={empty}><IconMapPinExclamation size={48} color="var(--color-warm-400)" /><h2 style={{ margin: 0 }}>Tidak ada laporan jamaah tersesat aktif</h2></section>
    ) : (
      <div style={list}>
        {active.map((report) => <article key={report.id} style={card}>
          <div style={row}>
            <div>
              <strong style={{ fontSize: 18 }}>{report.pilgrimName}</strong>
              <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)", fontSize: 13 }}>{report.groupName} · {report.createdAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</p>
              {report.pilgrimPhone && <p style={{ margin: "2px 0 0", color: "var(--color-warm-500)", fontSize: 13 }}>{report.pilgrimPhone}</p>}
            </div>
            <span style={badge}>{report.status === "LOST" ? "Tersesat" : report.status}</span>
          </div>
          {report.latitude !== 0 || report.longitude !== 0 ? (
            <a href={`https://www.google.com/maps?q=${report.latitude},${report.longitude}`} target="_blank" rel="noreferrer" style={mapLink}><IconMapPin size={15} />Lihat lokasi di peta</a>
          ) : (
            <p style={noLocation}><IconMapPin size={15} />Lokasi tidak tersedia</p>
          )}
          <div style={actions}>
            <button disabled={resolvingId === report.id} style={resolveButton} onClick={() => resolve(report.id)}>Tandai ditemukan</button>
          </div>
        </article>)}
      </div>
    )}
  </main>;
}

const page: React.CSSProperties = { maxWidth: 1000, margin: "0 auto", padding: "32px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const list: React.CSSProperties = { display: "grid", gap: 14 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-danger-600)", borderRadius: 12, padding: 18 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 12 };
const actions: React.CSSProperties = { display: "flex", gap: 10, marginTop: 14 };
const mapLink: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, marginTop: 10, color: "var(--color-emerald-900)", fontSize: 13, fontWeight: 600 };
const noLocation: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, marginTop: 10, color: "var(--color-warm-400)", fontSize: 13 };
const resolveButton: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-emerald-900)", color: "#fff", fontWeight: 700 };
const empty: React.CSSProperties = { minHeight: 280, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12 };
const badge: React.CSSProperties = { padding: "5px 10px", borderRadius: 99, background: "var(--color-danger-100)", color: "var(--color-danger-600)", fontSize: 12, fontWeight: 700 };
