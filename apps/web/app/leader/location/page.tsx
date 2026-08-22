"use client";

import { useState } from "react";
import { IconMapPin } from "@tabler/icons-react";
import { groupLeaderClient } from "@/lib/rpc";
import { useLeaderGroup } from "@/lib/leader-context";

const CITIES = [
  ["INDONESIA", "Indonesia"], ["TRANSIT", "Transit"], ["MADINAH", "Madinah"], ["MAKKAH", "Makkah"],
  ["ARAFAH", "Arafah"], ["MUZDALIFAH", "Muzdalifah"], ["MINA", "Mina"], ["DEPARTED", "Dalam Perjalanan Pulang"],
] as const;
const CITY_LABEL: Record<string, string> = Object.fromEntries(CITIES);

const ACTIVITY_PRESETS = [
  "City Tour", "Ziarah Masjid Quba", "Ziarah Masjid Qiblatain", "Ziarah Jabal Uhud",
  "Ziarah Makam Baqi", "Umrah Sunnah", "Waktu Bebas", "Persiapan Ihram",
];

export default function LeaderLocationPage() {
  const { groups, selectedGroupId: groupId, loaded } = useLeaderGroup();
  const group = groups.find((g) => g.id === groupId);
  const [city, setCity] = useState("");
  const [activity, setActivity] = useState("");
  const [location, setLocation] = useState("");
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");
  const [currentCity, setCurrentCity] = useState<string | undefined>(undefined);
  const [currentActivity, setCurrentActivity] = useState<string | undefined>(undefined);
  const [lastUpdate, setLastUpdate] = useState<Date | undefined>(undefined);

  const displayCity = currentCity ?? group?.currentCity ?? "INDONESIA";
  const displayActivity = currentActivity ?? group?.currentActivity;
  const displayLastUpdate = lastUpdate ?? group?.lastUpdate?.toDate();

  async function submit() {
    if (!groupId || !city) return;
    setSaving(true);
    setNotice("");
    try {
      const result = await groupLeaderClient.updateMyGroupCity({ groupId, city, activity, location, notes: "" });
      setCurrentCity(result.currentCity);
      setCurrentActivity(result.currentActivity);
      setLastUpdate(result.lastUpdate?.toDate());
      setCity("");
      setActivity("");
      setLocation("");
      setNotice("Lokasi grup berhasil diperbarui — jamaah dan operator otomatis mendapat notifikasi.");
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal memperbarui lokasi.");
    } finally {
      setSaving(false);
    }
  }

  if (!loaded) return <main style={page}><p style={{ color: "var(--color-warm-400)" }}>Memuat...</p></main>;

  return (
    <main style={page}>
      <p style={eyebrow}>LOKASI GRUP</p>
      <section style={currentCard}>
        <IconMapPin size={22} color="var(--color-emerald-800)" />
        <div>
          <p style={{ margin: 0, fontSize: 12, color: "var(--color-warm-400)" }}>Lokasi sekarang</p>
          <strong style={{ fontSize: 18 }}>{CITY_LABEL[displayCity] ?? displayCity}</strong>
          {displayActivity && <p style={{ margin: "2px 0 0", fontSize: 13, color: "var(--color-warm-600)" }}>{displayActivity}</p>}
          {displayLastUpdate && <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>Diperbarui {displayLastUpdate.toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</p>}
        </div>
      </section>

      {notice && <p style={noticeStyle}>{notice}</p>}

      <section style={form}>
        <label style={fieldLabel}>Update lokasi ke</label>
        <select value={city} onChange={(e) => setCity(e.target.value)} style={select}>
          <option value="">— pilih kota —</option>
          {CITIES.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </select>
        <label style={fieldLabel}>Aktivitas saat ini (opsional)</label>
        <div style={chipRow}>
          {ACTIVITY_PRESETS.map((preset) => (
            <button key={preset} type="button" onClick={() => setActivity(preset)} style={{ ...chip, ...(activity === preset ? chipActive : {}) }}>{preset}</button>
          ))}
        </div>
        <input value={activity} onChange={(e) => setActivity(e.target.value)} placeholder="mis. City Tour Jeddah, Ziarah Masjid Quba" style={input} />
        <label style={fieldLabel}>Detail lokasi (opsional)</label>
        <input value={location} onChange={(e) => setLocation(e.target.value)} placeholder="mis. Hotel Dar Al Hijra lantai 5" style={input} />
        <button disabled={saving || !city} onClick={() => void submit()} style={primary}>
          {saving ? "Menyimpan..." : "Update Lokasi Grup"}
        </button>
        <p style={{ margin: 0, fontSize: 12, color: "var(--color-warm-400)" }}>Satu tap ini otomatis memperbarui status perjalanan seluruh jamaah di grup Anda, dan terlihat oleh operator serta keluarga jamaah — tidak perlu langkah tambahan.</p>
      </section>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "20px 20px 0" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 12px" };
const currentCard: React.CSSProperties = { display: "flex", alignItems: "center", gap: 12, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16, marginBottom: 16 };
const noticeStyle: React.CSSProperties = { fontSize: 13, color: "var(--color-emerald-900)", background: "var(--color-emerald-50)", padding: "8px 12px", borderRadius: 8, marginBottom: 12 };
const form: React.CSSProperties = { display: "grid", gap: 10, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16 };
const fieldLabel: React.CSSProperties = { fontSize: 12, fontWeight: 600, color: "var(--color-warm-700)" };
const select: React.CSSProperties = { minHeight: 48, border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", font: "inherit", background: "#fff" };
const input: React.CSSProperties = { minHeight: 48, border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", font: "inherit" };
const primary: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, marginTop: 4 };
const chipRow: React.CSSProperties = { display: "flex", flexWrap: "wrap", gap: 6, marginBottom: 4 };
const chip: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 99, padding: "0 12px", background: "#fff", color: "var(--color-warm-600)", fontSize: 12, fontWeight: 600 };
const chipActive: React.CSSProperties = { background: "var(--color-emerald-900)", borderColor: "var(--color-emerald-900)", color: "#fff" };
