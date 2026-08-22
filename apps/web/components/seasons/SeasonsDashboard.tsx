"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { IconCalendar, IconCheck, IconPencil, IconPlus, IconReceiptRefund, IconTrash, IconUsers } from "@tabler/icons-react";
import { Season } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import { seasonClient } from "@/lib/rpc";
import { SEASON_TYPE_LABEL } from "@/lib/season-types";
import SeasonFormDialog from "./SeasonFormDialog";
import { RoleGate } from "@/components/auth/RoleGate";

export default function SeasonsDashboard() {
  const [seasons, setSeasons] = useState<Season[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [edit, setEdit] = useState<Season | undefined>();
  const [activating, setActivating] = useState("");
  const [deleting, setDeleting] = useState("");
  const [notice, setNotice] = useState("");

  const load = () => { setLoading(true); seasonClient.listSeasons({}).then((r) => setSeasons(r.seasons)).catch(() => setNotice("Gagal memuat daftar musim.")).finally(() => setLoading(false)); };

  useEffect(() => { void load(); }, []);

  async function activate(season: Season) {
    setActivating(season.id);
    try {
      await seasonClient.setActiveSeason({ seasonId: season.id });
      setNotice(`${season.name} sekarang menjadi musim aktif.`);
      void load();
    } catch (err) {
      setNotice(err instanceof Error ? err.message : "Gagal mengaktifkan musim.");
    } finally {
      setActivating("");
    }
  }

  async function remove(season: Season) {
    if (!window.confirm(`Hapus musim ${season.name}? Tindakan ini tidak dapat dibatalkan.`)) return;
    setDeleting(season.id);
    try {
      await seasonClient.deleteSeason({ seasonId: season.id });
      setNotice(`${season.name} berhasil dihapus.`);
      void load();
    } catch (err) {
      setNotice(err instanceof Error ? err.message : "Gagal menghapus musim.");
    } finally {
      setDeleting("");
    }
  }

  return (
    <main style={page}>
      <header style={header}>
        <div><p style={eyebrow}>OPERASIONAL / MUSIM</p><h1 style={title}>Musim</h1></div>
        <button style={gold} onClick={() => { setEdit(undefined); setOpen(true); }}><IconPlus size={18} />Tambah Musim</button>
      </header>
      <div className="gold-divider" />
      {notice && <p style={{ color: "var(--color-warm-500)" }}>{notice}</p>}
      {loading ? (
        <p style={{ color: "var(--color-warm-500)" }}>Memuat musim...</p>
      ) : seasons.length ? (
        <div style={grid}>
          {seasons.map((season) => (
            <article key={season.id} style={card}>
              <div style={row}>
                <h2 style={{ margin: 0, fontSize: 20 }}>{season.name}</h2>
                {season.isActive && <span style={activeBadge}><IconCheck size={13} />Aktif</span>}
              </div>
              <p style={typeLabel}>{SEASON_TYPE_LABEL[season.type] ?? "-"}</p>
              <p style={dateRange}>
                <IconCalendar size={15} />
                {season.startDate?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}
                {" – "}
                {season.endDate?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}
              </p>
              <p style={dateRange}><IconUsers size={15} />{season.capacity > 0 ? `Kapasitas ${season.capacity} jamaah` : "Kapasitas tidak dibatasi"}</p>
              <Link href={`/dashboard/seasons/${season.id}/cancellation-policy`} style={{ ...ghost, textDecoration: "none", display: "inline-flex", alignItems: "center", gap: 6, justifyContent: "center" }}><IconReceiptRefund size={15} />Atur Kebijakan Pembatalan</Link>
              {!season.isActive && (
                <button style={ghost} disabled={activating === season.id} onClick={() => void activate(season)}>
                  {activating === season.id ? "Mengaktifkan..." : "Jadikan Aktif"}
                </button>
              )}
              <div style={actionsRow}>
                <button style={iconGhost} onClick={() => { setEdit(season); setOpen(true); }}><IconPencil size={15} />Ubah</button>
                <RoleGate require={["owner", "admin"]}>
                  <button style={{ ...iconGhost, color: "var(--color-danger-600)" }} disabled={deleting === season.id} onClick={() => void remove(season)}><IconTrash size={15} />{deleting === season.id ? "Menghapus..." : "Hapus"}</button>
                </RoleGate>
              </div>
            </article>
          ))}
        </div>
      ) : (
        <section style={empty}>
          <IconCalendar size={48} color="var(--color-warm-400)" />
          <h2 style={{ margin: 0 }}>Belum ada musim</h2>
          <p style={{ color: "var(--color-warm-500)" }}>Buat musim untuk mulai mengelola jamaah, grup, dan operasional lainnya.</p>
          <button style={gold} onClick={() => setOpen(true)}>Tambah Musim</button>
        </section>
      )}
      <SeasonFormDialog open={open} initial={edit} onClose={() => setOpen(false)} onSaved={(name) => { setNotice(`${name} berhasil disimpan.`); void load(); }} />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 20, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const gold: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(280px,1fr))", gap: 16, marginTop: 24 };
const card: React.CSSProperties = { display: "grid", gap: 10, padding: 20, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 };
const typeLabel: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontWeight: 600, fontSize: 13 };
const dateRange: React.CSSProperties = { margin: 0, display: "flex", alignItems: "center", gap: 6, color: "var(--color-warm-500)", fontSize: 13 };
const activeBadge: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, padding: "4px 10px", borderRadius: 99, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 700 };
const ghost: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-emerald-900)", fontWeight: 600, fontSize: 13, marginTop: 4 };
const actionsRow: React.CSSProperties = { display: "flex", gap: 8, marginTop: 4, paddingTop: 10, borderTop: "1px solid var(--color-cream-300)" };
const iconGhost: React.CSSProperties = { border: 0, background: "transparent", display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-warm-500)", fontSize: 13 };
const empty: React.CSSProperties = { minHeight: 280, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12, marginTop: 24 };
