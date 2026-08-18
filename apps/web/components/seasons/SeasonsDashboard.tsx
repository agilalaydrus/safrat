"use client";
import { useEffect, useState } from "react";
import { IconCalendar, IconCheck, IconPlus } from "@tabler/icons-react";
import { Season } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import { seasonClient } from "@/lib/rpc";
import SeasonFormDialog from "./SeasonFormDialog";

const TYPE_LABEL: Record<number, string> = { 1: "Haji", 2: "Umrah" };

export default function SeasonsDashboard() {
  const [seasons, setSeasons] = useState<Season[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [activating, setActivating] = useState("");
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

  return (
    <main style={page}>
      <header style={header}>
        <div><p style={eyebrow}>OPERASIONAL / MUSIM</p><h1 style={title}>Musim</h1></div>
        <button style={gold} onClick={() => setOpen(true)}><IconPlus size={18} />Tambah Musim</button>
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
              <p style={typeLabel}>{TYPE_LABEL[season.type] ?? "-"}</p>
              <p style={dateRange}>
                <IconCalendar size={15} />
                {season.startDate?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}
                {" – "}
                {season.endDate?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}
              </p>
              {!season.isActive && (
                <button style={ghost} disabled={activating === season.id} onClick={() => void activate(season)}>
                  {activating === season.id ? "Mengaktifkan..." : "Jadikan Aktif"}
                </button>
              )}
            </article>
          ))}
        </div>
      ) : (
        <section style={empty}>
          <IconCalendar size={48} color="var(--color-warm-400)" />
          <h2 style={{ margin: 0 }}>Belum ada musim</h2>
          <p style={{ color: "var(--color-warm-500)" }}>Buat musim untuk mulai mengelola jamaah, rombongan, dan operasional lainnya.</p>
          <button style={gold} onClick={() => setOpen(true)}>Tambah Musim</button>
        </section>
      )}
      <SeasonFormDialog open={open} onClose={() => setOpen(false)} onSaved={(name) => { setNotice(`${name} berhasil dibuat.`); void load(); }} />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 20, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const gold: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "var(--color-gold-500)", color: "var(--color-warm-900)", fontWeight: 700 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(280px,1fr))", gap: 16, marginTop: 24 };
const card: React.CSSProperties = { display: "grid", gap: 10, padding: 20, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 };
const typeLabel: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontWeight: 600, fontSize: 13 };
const dateRange: React.CSSProperties = { margin: 0, display: "flex", alignItems: "center", gap: 6, color: "var(--color-warm-500)", fontSize: 13 };
const activeBadge: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, padding: "4px 10px", borderRadius: 99, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 700 };
const ghost: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-emerald-900)", fontWeight: 600, fontSize: 13, marginTop: 4 };
const empty: React.CSSProperties = { minHeight: 280, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12, marginTop: 24 };
