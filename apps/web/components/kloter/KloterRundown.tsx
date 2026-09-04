"use client";

import { useEffect, useMemo, useState } from "react";
import { IconDeviceFloppy, IconPlus, IconTrash } from "@tabler/icons-react";
import type { RundownItem } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { kloterClient } from "@/lib/rpc";

type Draft = { dayNumber: number; timeLabel: string; title: string; location: string; pic: string; notes: string };

const toDraft = (item: RundownItem): Draft => ({
  dayNumber: item.dayNumber, timeLabel: item.timeLabel, title: item.title,
  location: item.location, pic: item.pic, notes: item.notes,
});

export default function KloterRundown({ kloterId }: { kloterId: string }) {
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    setLoading(true);
    kloterClient.listKloterRundown({ kloterId })
      .then((r) => setDrafts(r.items.map(toDraft)))
      .catch(() => setFailure("Gagal memuat Rundown."))
      .finally(() => setLoading(false));
  }, [kloterId]);

  const byDay = useMemo(() => {
    const map = new Map<number, { item: Draft; index: number }[]>();
    drafts.forEach((item, index) => {
      const list = map.get(item.dayNumber) ?? [];
      list.push({ item, index });
      map.set(item.dayNumber, list);
    });
    return Array.from(map.entries()).sort((a, b) => a[0] - b[0]);
  }, [drafts]);

  const maxDay = drafts.reduce((m, d) => Math.max(m, d.dayNumber), 0);

  const update = (index: number, patch: Partial<Draft>) => {
    setDrafts(drafts.map((d, i) => (i === index ? { ...d, ...patch } : d)));
    setSaved(false);
  };
  const remove = (index: number) => { setDrafts(drafts.filter((_, i) => i !== index)); setSaved(false); };
  const addToDay = (dayNumber: number) => {
    setDrafts([...drafts, { dayNumber, timeLabel: "", title: "", location: "", pic: "", notes: "" }]);
    setSaved(false);
  };
  const addDay = () => addToDay(maxDay + 1 || 1);

  const allFilled = drafts.every((d) => d.title.trim() && d.dayNumber > 0);

  const save = async () => {
    setSaving(true);
    setFailure("");
    setSaved(false);
    try {
      await kloterClient.setKloterRundown({
        kloterId,
        items: drafts.map((d) => ({
          $typeName: "hajj.v1.RundownItemInput" as const,
          dayNumber: d.dayNumber, timeLabel: d.timeLabel, title: d.title,
          location: d.location, pic: d.pic, notes: d.notes,
        })),
      });
      setSaved(true);
    } catch (caught) {
      setFailure(caught instanceof Error ? caught.message : "Gagal menyimpan Rundown.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) return null;

  return (
    <section style={card}>
      <h2 style={sectionTitle}>Rundown Perjalanan</h2>
      <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-warm-500)" }}>Jadwal harian yang dipegang koordinator &amp; muttawwif di lapangan.</p>

      {byDay.length ? (
        <div style={{ display: "grid", gap: 16 }}>
          {byDay.map(([dayNumber, rows]) => (
            <div key={dayNumber}>
              <p style={dayLabel}>Hari {dayNumber}</p>
              <div style={{ display: "grid", gap: 8 }}>
                {rows.map(({ item, index }) => (
                  <div key={index} style={row}>
                    <input placeholder="Jam" value={item.timeLabel} onChange={(e) => update(index, { timeLabel: e.target.value })} style={timeInput} />
                    <input placeholder="Kegiatan" value={item.title} onChange={(e) => update(index, { title: e.target.value })} style={growInput} />
                    <input placeholder="Lokasi" value={item.location} onChange={(e) => update(index, { location: e.target.value })} style={growInput} />
                    <input placeholder="PIC" value={item.pic} onChange={(e) => update(index, { pic: e.target.value })} style={picInput} />
                    <button type="button" onClick={() => remove(index)} style={iconBtnDanger} aria-label="Hapus"><IconTrash size={14} /></button>
                  </div>
                ))}
              </div>
              <button type="button" onClick={() => addToDay(dayNumber)} style={ghostBtn}><IconPlus size={14} /> Tambah item Hari {dayNumber}</button>
            </div>
          ))}
        </div>
      ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada rundown.</p>}

      <button type="button" onClick={addDay} style={ghostBtn}><IconPlus size={14} /> Tambah Hari Baru</button>

      {failure && <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{failure}</p>}
      {saved && <p style={{ color: "var(--color-emerald-800)", fontSize: 13 }}>Rundown tersimpan.</p>}

      <button type="button" onClick={() => void save()} disabled={saving || !allFilled} style={primaryBtn}>
        <IconDeviceFloppy size={16} /> {saving ? "Menyimpan..." : "Simpan Rundown"}
      </button>
    </section>
  );
}

const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, marginTop: 16 };
const sectionTitle: React.CSSProperties = { margin: "0 0 4px", fontSize: 16 };
const dayLabel: React.CSSProperties = { margin: "0 0 8px", fontSize: 12, fontWeight: 700, color: "var(--color-gold-800)", textTransform: "uppercase", letterSpacing: ".06em" };
const row: React.CSSProperties = { display: "flex", gap: 8, alignItems: "center", padding: "8px 10px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, flexWrap: "wrap" };
const timeInput: React.CSSProperties = { flex: "0 1 80px", minHeight: 36, border: "1px solid var(--color-cream-400)", borderRadius: 6, padding: "0 8px", font: "inherit", background: "#fff" };
const growInput: React.CSSProperties = { flex: "1 1 160px", minHeight: 36, border: "1px solid var(--color-cream-400)", borderRadius: 6, padding: "0 8px", font: "inherit", background: "#fff" };
const picInput: React.CSSProperties = { flex: "0 1 120px", minHeight: 36, border: "1px solid var(--color-cream-400)", borderRadius: 6, padding: "0 8px", font: "inherit", background: "#fff" };
const iconBtnDanger: React.CSSProperties = { width: 30, height: 30, border: "1px solid var(--color-cream-400)", borderRadius: 6, background: "#fff", color: "var(--color-danger-600)", display: "grid", placeItems: "center" };
const ghostBtn: React.CSSProperties = { marginTop: 8, minHeight: 36, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 13, fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 6 };
const primaryBtn: React.CSSProperties = { marginTop: 14, minHeight: 44, border: 0, borderRadius: 8, padding: "0 18px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 8 };
