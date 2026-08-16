"use client";
import { useEffect, useState } from "react";
import { IconPencil, IconPlane, IconPlus, IconTrash } from "@tabler/icons-react";
import { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { kloterClient, seasonClient } from "@/lib/rpc";
import KloterFormDialog from "./KloterFormDialog";

export default function KloterDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [kloters, setKloters] = useState<Kloter[]>([]);
  const [open, setOpen] = useState(false);
  const [edit, setEdit] = useState<Kloter | undefined>();
  const [notice, setNotice] = useState("");

  const load = async (id = seasonId) => {
    if (!id) return;
    try { setKloters((await kloterClient.listKloters({ seasonId: id })).kloters); }
    catch { setNotice("Gagal memuat data kloter."); }
  };

  useEffect(() => {
    seasonClient.listSeasons({}).then((r) => { setSeasons(r.seasons); setSeasonId(r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? ""); }).catch(() => setNotice("Gagal memuat data musim."));
  }, []);
  useEffect(() => { void load(); }, [seasonId]);

  async function remove(kloter: Kloter) {
    if (!window.confirm(`Hapus kloter ${kloter.code}? Jamaah akan dilepas dari kloter ini, bukan dihapus.`)) return;
    await kloterClient.deleteKloter({ kloterId: kloter.id });
    void load();
  }

  return (
    <main style={page}>
      <header style={header}>
        <div><p style={eyebrow}>OPERASIONAL / KLOTER</p><h1 style={title}>Kloter Keberangkatan</h1></div>
        <div style={actions}>
          <select value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
            {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
          <button style={emerald} onClick={() => { setEdit(undefined); setOpen(true); }}><IconPlus size={18} />Tambah Kloter</button>
        </div>
      </header>
      <div className="gold-divider" />
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      {kloters.length ? (
        <div style={grid}>
          {kloters.map((k) => (
            <article style={card} key={k.id}>
              <div style={row}><h2 style={{ margin: 0, fontSize: 18 }}>{k.code}</h2><span style={capBadge}>{k.pilgrimCount}{k.capacity ? `/${k.capacity}` : ""}</span></div>
              <p style={meta}>{k.embarkation || "Embarkasi belum ditentukan"}</p>
              <p style={meta}><IconPlane size={14} style={{ verticalAlign: "-2px", marginRight: 4 }} />{k.flightNumber || "Nomor penerbangan belum ditentukan"}</p>
              {k.departureDate && <p style={meta}>{k.departureDate.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "long", year: "numeric" })}</p>}
              <div style={row}>
                <button style={ghost} onClick={() => { setEdit(k); setOpen(true); }}><IconPencil size={15} />Ubah</button>
                <button style={{ ...ghost, color: "var(--color-danger-600)" }} onClick={() => void remove(k)}><IconTrash size={15} />Hapus</button>
              </div>
            </article>
          ))}
        </div>
      ) : (
        <section style={empty}>
          <IconPlane size={48} color="var(--color-warm-400)" />
          <h2 style={{ margin: 0 }}>Belum ada kloter</h2>
          <p style={{ margin: 0, color: "var(--color-warm-500)" }}>Buat kloter untuk mengelompokkan jamaah berdasarkan jadwal keberangkatan.</p>
          <button style={gold} onClick={() => setOpen(true)}>Tambah Kloter</button>
        </section>
      )}
      <KloterFormDialog open={open} seasonId={seasonId} initial={edit} onClose={() => setOpen(false)} onSaved={(code) => { setNotice(`Kloter ${code} berhasil disimpan.`); void load(); }} />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 20, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const actions: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap" };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "#fff", maxWidth: 220 };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "var(--color-emerald-900)", color: "var(--color-cream-100)", fontWeight: 700 };
const gold: React.CSSProperties = { ...emerald, background: "var(--color-gold-500)", color: "var(--color-warm-900)" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(260px,1fr))", gap: 16 };
const card: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, display: "grid", gap: 8 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 };
const capBadge: React.CSSProperties = { padding: "4px 8px", borderRadius: 99, background: "var(--color-gold-50)", color: "var(--color-gold-800)", fontSize: 12 };
const meta: React.CSSProperties = { margin: 0, color: "var(--color-warm-500)", fontSize: 13 };
const ghost: React.CSSProperties = { border: 0, background: "transparent", display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-warm-500)", marginRight: 8 };
const empty: React.CSSProperties = { minHeight: 280, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12, textAlign: "center", padding: 24 };
