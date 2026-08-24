"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { IconCheck, IconGenderFemale, IconGenderMale, IconMapPin, IconMapPinExclamation, IconPlane, IconSos, IconUsersGroup, IconWheelchair, IconWifiOff, IconX } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { LostReport } from "@hajj-saas/proto-gen/hajj/v1/lost_report_pb";
import { SOSAlert } from "@hajj-saas/proto-gen/hajj/v1/sos_pb";
import { groupLeaderClient, kloterClient, lostReportClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";
import { useLeaderGroup } from "@/lib/leader-context";

const PAYMENT_LABEL: Record<string, { label: string; color: string; bg: string }> = {
  UNPAID: { label: "Belum Bayar", color: "var(--color-danger-600)", bg: "var(--color-danger-100)" },
  DP: { label: "DP", color: "var(--color-gold-800)", bg: "var(--color-gold-50)" },
  PAID: { label: "Lunas", color: "var(--color-emerald-900)", bg: "var(--color-emerald-50)" },
};

// "Terkonfirmasi" means both identity (KYC) is verified by admin AND
// payment has started (DP or Lunas) — neither alone is enough, and being
// merely "not cancelled" was never a meaningful confirmation signal.
function confirmationState(status: string, kycStatus: string, paymentStatus: string): "CANCELLED" | "CONFIRMED" | "PENDING" {
  if (status === "CANCELLED") return "CANCELLED";
  if (kycStatus === "VERIFIED" && paymentStatus !== "UNPAID") return "CONFIRMED";
  return "PENDING";
}

export default function LeaderRosterPage() {
  const { groups, loaded, selectedGroupId } = useLeaderGroup();
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [kloterCodes, setKloterCodes] = useState<Record<string, string>>({});
  const [fromCache, setFromCache] = useState(false);
  const [error, setError] = useState("");
  const [lostReports, setLostReports] = useState<LostReport[]>([]);
  const [resolvingLostId, setResolvingLostId] = useState("");
  const [sosAlerts, setSosAlerts] = useState<SOSAlert[]>([]);
  const [ackingSosId, setAckingSosId] = useState("");

  useEffect(() => {
    if (!selectedGroupId) return;
    cachedFetch(`leader-roster:${selectedGroupId}`, () => groupLeaderClient.getGroupRoster({ groupId: selectedGroupId }))
      .then((result) => {
        if (result.data) setPilgrims(result.data.pilgrims);
        setFromCache(result.fromCache);
      })
      .catch(() => setError("Gagal memuat daftar jamaah grup ini."));
  }, [selectedGroupId]);

  const refreshLost = useCallback(() => {
    if (!selectedGroupId) return;
    lostReportClient.listGroupLostReports({ groupId: selectedGroupId }).then((response) => {
      setLostReports(response.reports.filter((r) => r.status !== "RESOLVED"));
    }).catch(() => {});
  }, [selectedGroupId]);

  useEffect(() => {
    if (!selectedGroupId) return;
    refreshLost();
    const interval = window.setInterval(refreshLost, 10000);
    return () => window.clearInterval(interval);
  }, [refreshLost, selectedGroupId]);

  async function resolveLost(id: string) {
    if (!selectedGroupId) return;
    setResolvingLostId(id);
    try { await lostReportClient.resolveGroupLostReport({ groupId: selectedGroupId, id }); refreshLost(); } catch { setError("Gagal menandai laporan sebagai ditemukan."); } finally { setResolvingLostId(""); }
  }

  const refreshSos = useCallback(() => {
    groupLeaderClient.listMySOSAlerts({}).then((response) => {
      setSosAlerts(response.alerts.filter((a) => a.status !== "RESOLVED"));
    }).catch(() => {});
  }, []);

  useEffect(() => {
    refreshSos();
    const interval = window.setInterval(refreshSos, 10000);
    return () => window.clearInterval(interval);
  }, [refreshSos]);

  async function acknowledgeSos(id: string) {
    setAckingSosId(id);
    try { await groupLeaderClient.acknowledgeMySOSAlert({ sosAlertId: id }); refreshSos(); } catch { setError("Gagal mengonfirmasi SOS."); } finally { setAckingSosId(""); }
  }

  useEffect(() => {
    if (!selectedGroupId) return;
    const seasonId = groups.find((g) => g.id === selectedGroupId)?.seasonId;
    if (!seasonId) return;
    kloterClient.listKloters({ seasonId }).then((r) => setKloterCodes(Object.fromEntries(r.kloters.map((k) => [k.id, k.code])))).catch(() => {});
  }, [selectedGroupId, groups]);

  if (loaded && !groups.length) {
    return (
      <main style={page}>
        <section style={empty}>
          <IconUsersGroup size={44} color="var(--color-warm-400)" />
          <p style={{ color: "var(--color-warm-500)" }}>Anda belum ditugaskan sebagai ketua grup mana pun. Hubungi operator Anda untuk penugasan.</p>
        </section>
      </main>
    );
  }

  return (
    <main style={page}>
      <p style={eyebrow}>DAFTAR JAMAAH</p>
      <h1 style={title}>{pilgrims.length} jamaah</h1>
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Menampilkan data tersimpan — Anda sedang offline</p>}
      {error && <p style={{ color: "var(--color-danger-600)" }}>{error}</p>}
      {sosAlerts.map((alert) => (
        <div key={alert.id} style={sosBanner}>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <IconSos size={22} color="#fff" />
            <div>
              <p style={{ margin: 0, fontWeight: 700 }}>{alert.pilgrimName} mengirim SOS{alert.status === "ESCALATED" ? " — dieskalasi ke petugas" : ""}</p>
              <p style={{ margin: "2px 0 0", fontSize: 12, opacity: 0.9 }}>{alert.status === "ACKNOWLEDGED" ? "Anda sedang menuju lokasi" : "Butuh konfirmasi segera"}</p>
            </div>
          </div>
          <div style={{ display: "flex", gap: 8, marginTop: 10 }}>
            {alert.hasLocation && <a href={`https://www.google.com/maps?q=${alert.lat},${alert.lng}`} target="_blank" rel="noreferrer" style={bannerGhostButton}><IconMapPin size={14} />Lokasi</a>}
            {alert.status !== "ACKNOWLEDGED" && <button disabled={ackingSosId === alert.id} onClick={() => acknowledgeSos(alert.id)} style={bannerSolidButton}><IconCheck size={14} />Konfirmasi</button>}
            <Link href="/leader/sos" style={bannerGhostButton}>Detail</Link>
          </div>
        </div>
      ))}
      {lostReports.map((report) => (
        <div key={report.id} style={lostBanner}>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <IconMapPinExclamation size={22} color="#fff" />
            <div>
              <p style={{ margin: 0, fontWeight: 700 }}>{report.pilgrimName} melaporkan tersesat</p>
              <p style={{ margin: "2px 0 0", fontSize: 12, opacity: 0.9 }}>Tandai ditemukan setelah jamaah kembali bersama grup</p>
            </div>
          </div>
          <div style={{ display: "flex", gap: 8, marginTop: 10 }}>
            <a href={`https://www.google.com/maps?q=${report.latitude},${report.longitude}`} target="_blank" rel="noreferrer" style={bannerGhostButton}><IconMapPin size={14} />Lokasi</a>
            <button disabled={resolvingLostId === report.id} onClick={() => resolveLost(report.id)} style={bannerSolidButton}><IconCheck size={14} />Tandai ditemukan</button>
          </div>
        </div>
      ))}
      <div style={list}>
        {pilgrims.map((pilgrim) => (
          <article key={pilgrim.id} style={card}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <strong>{pilgrim.fullName}</strong>
              <p style={meta}>{pilgrim.gender === Gender.FEMALE ? <IconGenderFemale size={15} /> : <IconGenderMale size={15} />}{pilgrim.passportNumber}{pilgrim.kloterId && kloterCodes[pilgrim.kloterId] && <span style={kloterMeta}><IconPlane size={13} />{kloterCodes[pilgrim.kloterId]}</span>}</p>
              <div style={badgeRow}>
                {(() => {
                  const state = confirmationState(pilgrim.status, pilgrim.kycStatus, pilgrim.paymentStatus);
                  if (state === "CANCELLED") return <span style={badgeCancelled}><IconX size={12} />Dibatalkan</span>;
                  if (state === "CONFIRMED") return <span style={badgeConfirmed}><IconCheck size={12} />Terkonfirmasi</span>;
                  return <span style={badgePending}>Menunggu Konfirmasi</span>;
                })()}
                <span style={{ ...badgeBase, color: (PAYMENT_LABEL[pilgrim.paymentStatus] ?? PAYMENT_LABEL.UNPAID!).color, background: (PAYMENT_LABEL[pilgrim.paymentStatus] ?? PAYMENT_LABEL.UNPAID!).bg }}>
                  {(PAYMENT_LABEL[pilgrim.paymentStatus] ?? PAYMENT_LABEL.UNPAID!).label}
                </span>
              </div>
            </div>
            {pilgrim.requiresWheelchair && <IconWheelchair size={20} color="var(--color-gold-800)" />}
          </article>
        ))}
      </div>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "20px 20px 0" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 26, margin: "0 0 14px" };
const offlineBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, background: "var(--color-gold-50)", color: "var(--color-gold-800)", padding: "8px 12px", borderRadius: 8, fontSize: 12, marginBottom: 14 };
const lostBanner: React.CSSProperties = { background: "var(--color-danger-600)", color: "#fff", padding: "12px 14px", borderRadius: 12, marginBottom: 10 };
const sosBanner: React.CSSProperties = { background: "var(--color-danger-700, #991b1b)", color: "#fff", padding: "12px 14px", borderRadius: 12, marginBottom: 10, boxShadow: "0 6px 18px rgba(220,38,38,.35)" };
const bannerGhostButton: React.CSSProperties = { minHeight: 34, border: "1px solid rgba(255,255,255,.6)", borderRadius: 8, padding: "0 12px", background: "transparent", color: "#fff", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12, fontWeight: 600, textDecoration: "none" };
const bannerSolidButton: React.CSSProperties = { minHeight: 34, border: 0, borderRadius: 8, padding: "0 12px", background: "#fff", color: "var(--color-danger-600)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12, fontWeight: 700 };
const list: React.CSSProperties = { display: "grid", gap: 10 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14, display: "flex", justifyContent: "space-between", alignItems: "center" };
const meta: React.CSSProperties = { margin: "4px 0 0", display: "flex", alignItems: "center", gap: 6, color: "var(--color-warm-500)", fontSize: 12 };
const kloterMeta: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 3, marginLeft: 8 };
const empty: React.CSSProperties = { minHeight: 300, display: "grid", placeItems: "center", alignContent: "center", gap: 10, textAlign: "center", padding: 20 };
const badgeRow: React.CSSProperties = { display: "flex", gap: 6, marginTop: 8, flexWrap: "wrap" };
const badgeBase: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, padding: "3px 8px", borderRadius: 99, fontSize: 11, fontWeight: 700 };
const badgeConfirmed: React.CSSProperties = { ...badgeBase, color: "var(--color-emerald-900)", background: "var(--color-emerald-50)" };
const badgeCancelled: React.CSSProperties = { ...badgeBase, color: "var(--color-danger-600)", background: "var(--color-danger-100)" };
const badgePending: React.CSSProperties = { ...badgeBase, color: "var(--color-gold-800)", background: "var(--color-gold-50)" };
