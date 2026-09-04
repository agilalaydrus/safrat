"use client";

import { useEffect, useState } from "react";
import { IconArrowDown, IconArrowUp, IconBus, IconBuilding, IconDeviceFloppy, IconPlane, IconTrash } from "@tabler/icons-react";
import type { ItinerarySegment } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import type { Movement } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import type { Hotel } from "@hajj-saas/proto-gen/hajj/v1/accommodation_pb";
import { kloterClient, transportClient, accommodationClient } from "@/lib/rpc";

const MODE_ICON: Record<string, React.ComponentType<{ size?: number }>> = { BUS: IconBus, FLIGHT: IconPlane, TRAIN: IconBus };

type Draft = { segmentType: "TRANSPORT" | "HOTEL"; movementId: string; hotelId: string; notes: string };

const toDraft = (seg: ItinerarySegment): Draft => ({
  segmentType: seg.segmentType === "HOTEL" ? "HOTEL" : "TRANSPORT",
  movementId: seg.movementId,
  hotelId: seg.hotelId,
  notes: seg.notes,
});

export default function KloterItinerary({ kloterId, seasonId }: { kloterId: string; seasonId: string }) {
  const [movements, setMovements] = useState<Movement[]>([]);
  const [hotels, setHotels] = useState<Hotel[]>([]);
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      kloterClient.listKloterItinerary({ kloterId }).then((r) => setDrafts(r.segments.map(toDraft))),
      transportClient.listMovements({ seasonId }).then((r) => setMovements(r.movements.filter((m) => m.kloterId === kloterId))),
      accommodationClient.listHotels({ seasonId }).then((r) => setHotels(r.hotels)),
    ]).catch(() => setFailure("Gagal memuat Rangkaian.")).finally(() => setLoading(false));
  }, [kloterId, seasonId]);

  const movementName = (id: string) => movements.find((m) => m.id === id)?.name || "— pilih segmen Transportasi —";
  const hotelName = (id: string) => {
    const h = hotels.find((x) => x.id === id);
    return h ? `${h.name} (${h.city})` : "— pilih Hotel —";
  };

  const move = (i: number, dir: -1 | 1) => {
    const j = i + dir;
    if (j < 0 || j >= drafts.length) return;
    const next = [...drafts];
    const a = next[i]!, b = next[j]!;
    next[i] = b; next[j] = a;
    setDrafts(next);
    setSaved(false);
  };
  const remove = (i: number) => { setDrafts(drafts.filter((_, idx) => idx !== i)); setSaved(false); };
  const add = (segmentType: "TRANSPORT" | "HOTEL") => {
    setDrafts([...drafts, { segmentType, movementId: "", hotelId: "", notes: "" }]);
    setSaved(false);
  };
  const update = (i: number, patch: Partial<Draft>) => {
    setDrafts(drafts.map((d, idx) => (idx === i ? { ...d, ...patch } : d)));
    setSaved(false);
  };

  const bookendsOk = drafts.length === 0 || (drafts[0]!.segmentType === "TRANSPORT" && drafts[drafts.length - 1]!.segmentType === "TRANSPORT");
  const allFilled = drafts.every((d) => (d.segmentType === "TRANSPORT" ? d.movementId : d.hotelId));

  const save = async () => {
    setSaving(true);
    setFailure("");
    setSaved(false);
    try {
      await kloterClient.setKloterItinerary({
        kloterId,
        segments: drafts.map((d) => ({
          $typeName: "hajj.v1.ItinerarySegmentInput" as const,
          segmentType: d.segmentType, movementId: d.movementId, hotelId: d.hotelId, notes: d.notes,
        })),
      });
      setSaved(true);
    } catch (caught) {
      setFailure(caught instanceof Error ? caught.message : "Gagal menyimpan Rangkaian.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) return null;

  return (
    <section style={card}>
      <h2 style={sectionTitle}>Rangkaian Perjalanan</h2>
      <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-warm-500)" }}>
        Urut sesuai ditambahkan — mulai &amp; akhiri dengan Transportasi.
      </p>
      {!movements.length && <p style={warn}>Belum ada jadwal Transportasi untuk kloter ini — tambahkan dulu di menu Transportasi.</p>}

      {drafts.length ? (
        <div style={{ display: "grid", gap: 8 }}>
          {drafts.map((d, i) => {
            const Icon = d.segmentType === "TRANSPORT" ? (MODE_ICON[movements.find((m) => m.id === d.movementId)?.mode ?? ""] ?? IconBus) : IconBuilding;
            return (
              <div key={i} style={row}>
                <Icon size={16} />
                <span style={typeBadge}>{d.segmentType === "TRANSPORT" ? "Transportasi" : "Hotel"}</span>
                {d.segmentType === "TRANSPORT" ? (
                  <select value={d.movementId} onChange={(e) => update(i, { movementId: e.target.value })} style={select}>
                    <option value="">{movementName("")}</option>
                    {movements.map((m) => <option key={m.id} value={m.id}>{m.name}</option>)}
                  </select>
                ) : (
                  <select value={d.hotelId} onChange={(e) => update(i, { hotelId: e.target.value })} style={select}>
                    <option value="">{hotelName("")}</option>
                    {hotels.map((h) => <option key={h.id} value={h.id}>{h.name} ({h.city})</option>)}
                  </select>
                )}
                <input placeholder="Catatan (opsional)" value={d.notes} onChange={(e) => update(i, { notes: e.target.value })} style={noteInput} />
                <button type="button" onClick={() => move(i, -1)} disabled={i === 0} style={iconBtn} aria-label="Naikkan"><IconArrowUp size={14} /></button>
                <button type="button" onClick={() => move(i, 1)} disabled={i === drafts.length - 1} style={iconBtn} aria-label="Turunkan"><IconArrowDown size={14} /></button>
                <button type="button" onClick={() => remove(i)} style={iconBtnDanger} aria-label="Hapus"><IconTrash size={14} /></button>
              </div>
            );
          })}
        </div>
      ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada segmen. Tambahkan segmen pertama — harus Transportasi.</p>}

      <div style={{ display: "flex", gap: 8, marginTop: 12, flexWrap: "wrap" }}>
        <button type="button" onClick={() => add("TRANSPORT")} style={ghostBtn} disabled={!movements.length}><IconBus size={14} /> Tambah Transportasi</button>
        <button type="button" onClick={() => add("HOTEL")} style={ghostBtn} disabled={!hotels.length}><IconBuilding size={14} /> Tambah Hotel</button>
      </div>

      {!bookendsOk && <p style={warn}>Rangkaian harus mulai &amp; akhiri dengan segmen Transportasi.</p>}
      {failure && <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{failure}</p>}
      {saved && <p style={{ color: "var(--color-emerald-800)", fontSize: 13 }}>Rangkaian tersimpan.</p>}

      <button type="button" onClick={() => void save()} disabled={saving || !bookendsOk || !allFilled} style={primaryBtn}>
        <IconDeviceFloppy size={16} /> {saving ? "Menyimpan..." : "Simpan Rangkaian"}
      </button>
    </section>
  );
}

const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, marginTop: 16 };
const sectionTitle: React.CSSProperties = { margin: "0 0 4px", fontSize: 16 };
const row: React.CSSProperties = { display: "flex", gap: 8, alignItems: "center", padding: "8px 10px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, flexWrap: "wrap" };
const typeBadge: React.CSSProperties = { flexShrink: 0, padding: "3px 8px", borderRadius: 99, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 10, fontWeight: 700, whiteSpace: "nowrap" };
const select: React.CSSProperties = { flex: "1 1 200px", minHeight: 36, border: "1px solid var(--color-cream-400)", borderRadius: 6, padding: "0 8px", font: "inherit", background: "#fff" };
const noteInput: React.CSSProperties = { flex: "1 1 160px", minHeight: 36, border: "1px solid var(--color-cream-400)", borderRadius: 6, padding: "0 8px", font: "inherit", background: "#fff" };
const iconBtn: React.CSSProperties = { width: 30, height: 30, border: "1px solid var(--color-cream-400)", borderRadius: 6, background: "#fff", color: "var(--color-warm-500)", display: "grid", placeItems: "center" };
const iconBtnDanger: React.CSSProperties = { ...iconBtn, color: "var(--color-danger-600)" };
const ghostBtn: React.CSSProperties = { minHeight: 36, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 13, fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 6 };
const primaryBtn: React.CSSProperties = { marginTop: 14, minHeight: 44, border: 0, borderRadius: 8, padding: "0 18px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 8 };
const warn: React.CSSProperties = { margin: "8px 0", fontSize: 13, color: "var(--color-danger-600)" };
