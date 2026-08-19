"use client";

import { useEffect, useState } from "react";
import { IconArrowUp, IconClock, IconLink, IconTrash } from "@tabler/icons-react";
import { WaitlistEntry } from "@hajj-saas/proto-gen/hajj/v1/waitlist_pb";
import { operatorClient, seasonClient, waitlistClient } from "@/lib/rpc";
import { RoleGate } from "@/components/auth/RoleGate";

const STATUS_LABEL: Record<string, string> = { WAITING: "Menunggu", PROMOTED: "Ditawarkan", CONFIRMED: "Dikonfirmasi", EXPIRED: "Kedaluwarsa", REMOVED: "Dihapus" };
const STATUS_COLOR: Record<string, string> = { WAITING: "var(--color-gold-800)", PROMOTED: "var(--color-emerald-800)", CONFIRMED: "var(--color-emerald-900)", EXPIRED: "var(--color-danger-600)", REMOVED: "var(--color-warm-400)" };

export default function WaitlistDashboard() {
  const [operatorId, setOperatorId] = useState("");
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [entries, setEntries] = useState<WaitlistEntry[]>([]);
  const [totalWaiting, setTotalWaiting] = useState(0);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [working, setWorking] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    operatorClient.getMyOperator({}).then((response) => setOperatorId(response.id)).catch(() => {});
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      setSeasonId(response.seasons.find((s) => s.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat data musim."));
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    setLoading(true);
    waitlistClient.listWaitlist({ seasonId }).then((response) => { setEntries(response.entries); setTotalWaiting(response.totalWaiting); }).catch(() => setNotice("Gagal memuat daftar tunggu.")).finally(() => setLoading(false));
  };
  useEffect(refresh, [seasonId]);

  const activeName = seasons.find((s) => s.id === seasonId)?.name ?? "Pilih musim";
  const publicLink = operatorId && seasonId ? `${typeof window !== "undefined" ? window.location.origin : ""}/waitlist/${operatorId}/${seasonId}` : "";

  const copyLink = async () => {
    if (!publicLink) return;
    await navigator.clipboard.writeText(publicLink);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2000);
  };

  const promote = async (entry: WaitlistEntry) => {
    setWorking(entry.id);
    setNotice("");
    try {
      await waitlistClient.promoteFromWaitlist({ id: entry.id });
      setNotice(`${entry.fullName} ditawarkan slot. Menunggu konfirmasi dalam 48 jam.`);
      refresh();
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setWorking("");
    }
  };

  const remove = async (entry: WaitlistEntry) => {
    if (!window.confirm(`Hapus ${entry.fullName} dari daftar tunggu?`)) return;
    setWorking(entry.id);
    setNotice("");
    try {
      await waitlistClient.removeFromWaitlist({ id: entry.id });
      setNotice(`${entry.fullName} dihapus dari daftar tunggu.`);
      refresh();
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setWorking("");
    }
  };

  return <main style={page}>
    <header style={header}>
      <div><p style={eyebrow}>OPERASIONAL / DAFTAR TUNGGU</p><h1 style={title}>Daftar Tunggu</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>{`${totalWaiting} menunggu · ${entries.length} total${activeName ? ` · ${activeName}` : ""}`}</p></div>
      <select aria-label="Musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
        {seasons.length ? seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>) : <option>{activeName}</option>}
      </select>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
    {publicLink && <section style={linkCard}>
      <IconLink size={18} />
      <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", flex: 1 }}>{publicLink}</span>
      <button onClick={copyLink} style={copyButton}>{copied ? "Tersalin!" : "Salin tautan"}</button>
    </section>}
    <section style={{ marginTop: 20 }}>
      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : entries.length ? <div style={{ overflowX: "auto" }}>
        <table style={table}>
          <thead><tr>{["Posisi", "Nama", "Email", "Telepon", "Status", "Aksi"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
          <tbody>
            {entries.map((entry) => <tr key={entry.id} style={tr}>
              <td style={td}>#{entry.position}</td>
              <td style={td}><strong>{entry.fullName}</strong></td>
              <td style={{ ...td, color: "var(--color-warm-500)" }}>{entry.email}</td>
              <td style={{ ...td, color: "var(--color-warm-500)" }}>{entry.phone || "-"}</td>
              <td style={td}><span style={{ color: STATUS_COLOR[entry.status] ?? "var(--color-warm-500)", fontWeight: 700, fontSize: 12 }}>{STATUS_LABEL[entry.status] ?? entry.status}</span>{entry.status === "PROMOTED" && entry.expiresAt && <span style={{ display: "block", fontSize: 11, color: "var(--color-warm-400)" }}>s.d. {entry.expiresAt.toDate().toLocaleString("id-ID")}</span>}</td>
              <td style={td}>
                <RoleGate require={["owner", "admin"]}>
                  <div style={{ display: "flex", gap: 8 }}>
                    {entry.status === "WAITING" && <button disabled={working === entry.id} onClick={() => promote(entry)} style={promoteBtn}><IconArrowUp size={14} />Tawarkan</button>}
                    {(entry.status === "WAITING" || entry.status === "PROMOTED") && <button disabled={working === entry.id} onClick={() => remove(entry)} style={removeBtn}><IconTrash size={14} /></button>}
                  </div>
                </RoleGate>
              </td>
            </tr>)}
          </tbody>
        </table>
      </div> : <div style={empty}><IconClock size={48} color="var(--color-warm-400)" /><p style={{ color: "var(--color-warm-500)" }}>Belum ada yang masuk daftar tunggu untuk musim ini.</p></div>}
    </section>
  </main>;
}

const page: React.CSSProperties = { maxWidth: 1100, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)" };
const linkCard: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "10px 14px", margin: "16px 0", color: "var(--color-warm-700)", fontSize: 13 };
const copyButton: React.CSSProperties = { minHeight: 36, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 14px", flexShrink: 0 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", minWidth: 720, background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, overflow: "hidden" };
const th: React.CSSProperties = { background: "var(--color-cream-200)", padding: "14px 16px", textAlign: "start", fontSize: 11, textTransform: "uppercase", letterSpacing: ".08em", color: "var(--color-warm-400)" };
const tr: React.CSSProperties = { borderTop: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: "14px 16px", fontSize: 14 };
const promoteBtn: React.CSSProperties = { minHeight: 36, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 12px", fontSize: 12, display: "inline-flex", gap: 4, alignItems: "center" };
const removeBtn: React.CSSProperties = { minHeight: 36, minWidth: 36, border: "1px solid var(--color-danger-600)", borderRadius: 8, background: "transparent", color: "var(--color-danger-600)", display: "grid", placeItems: "center" };
const empty: React.CSSProperties = { padding: "64px 24px", textAlign: "center", display: "grid", justifyItems: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12 };
