"use client";

import { useEffect, useState } from "react";
import { IconBellRinging, IconMoon } from "@tabler/icons-react";
import { notificationSettingsClient } from "@/lib/rpc";

export default function NotificationSettingsPanel() {
  const [quietEnabled, setQuietEnabled] = useState(false);
  const [start, setStart] = useState("22:00");
  const [end, setEnd] = useState("06:00");
  const [notifyGroupCity, setNotifyGroupCity] = useState(true);
  const [notifyKloterStatus, setNotifyKloterStatus] = useState(true);
  const [notifyRitual, setNotifyRitual] = useState(true);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    notificationSettingsClient.getNotificationSettings({}).then((s) => {
      setQuietEnabled(s.quietHoursEnabled);
      setStart(s.quietHoursStart || "22:00");
      setEnd(s.quietHoursEnd || "06:00");
      setNotifyGroupCity(s.notifyGroupCityChange);
      setNotifyKloterStatus(s.notifyKloterStatusChange);
      setNotifyRitual(s.notifyRitualBulkComplete);
    }).catch(() => setError("Gagal memuat pengaturan notifikasi.")).finally(() => setLoading(false));
  }, []);

  const save = async () => {
    setError(""); setNotice(""); setSaving(true);
    try {
      const s = await notificationSettingsClient.setNotificationSettings({
        quietHoursEnabled: quietEnabled, quietHoursStart: start, quietHoursEnd: end,
        notifyGroupCityChange: notifyGroupCity, notifyKloterStatusChange: notifyKloterStatus, notifyRitualBulkComplete: notifyRitual,
      });
      setStart(s.quietHoursStart);
      setEnd(s.quietHoursEnd);
      setNotice("Pengaturan notifikasi disimpan.");
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menyimpan pengaturan.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <p style={{ color: "var(--color-warm-400)" }}>Memuat...</p>;

  return (
    <div style={wrap}>
      <section style={card}>
        <h2 style={{ margin: "0 0 4px", display: "flex", alignItems: "center", gap: 8 }}><IconMoon size={20} />Jam Tenang</h2>
        <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-warm-500)" }}>
          Tunda notifikasi rutin (bukan darurat) selama jam ini. Peringatan kesehatan berat tetap terkirim kapan pun.
        </p>
        <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, fontWeight: 600, marginBottom: 12 }}>
          <input type="checkbox" checked={quietEnabled} onChange={(e) => setQuietEnabled(e.target.checked)} />
          Aktifkan jam tenang
        </label>
        {quietEnabled && (
          <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
            <label style={label1}><span>Mulai</span><input type="time" style={input} value={start} onChange={(e) => setStart(e.target.value)} /></label>
            <span style={{ color: "var(--color-warm-400)", marginTop: 18 }}>sampai</span>
            <label style={label1}><span>Selesai</span><input type="time" style={input} value={end} onChange={(e) => setEnd(e.target.value)} /></label>
          </div>
        )}
      </section>

      <section style={card}>
        <h2 style={{ margin: "0 0 4px", display: "flex", alignItems: "center", gap: 8 }}><IconBellRinging size={20} />Notifikasi ke Jamaah</h2>
        <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-warm-500)" }}>Pilih pembaruan otomatis mana yang dikirim sebagai notifikasi.</p>
        <div style={{ display: "grid", gap: 10 }}>
          <label style={toggleRow}>
            <input type="checkbox" checked={notifyGroupCity} onChange={(e) => setNotifyGroupCity(e.target.checked)} />
            <span><strong>Perpindahan kota grup</strong><br /><span style={{ fontSize: 12, color: "var(--color-warm-500)" }}>Saat grup pindah kota (mis. Madinah → Makkah)</span></span>
          </label>
          <label style={toggleRow}>
            <input type="checkbox" checked={notifyKloterStatus} onChange={(e) => setNotifyKloterStatus(e.target.checked)} />
            <span><strong>Status kloter</strong><br /><span style={{ fontSize: 12, color: "var(--color-warm-500)" }}>Saat status keberangkatan/kepulangan kloter berubah</span></span>
          </label>
          <label style={toggleRow}>
            <input type="checkbox" checked={notifyRitual} onChange={(e) => setNotifyRitual(e.target.checked)} />
            <span><strong>Ritual selesai (massal)</strong><br /><span style={{ fontSize: 12, color: "var(--color-warm-500)" }}>Saat petugas menandai ritual selesai untuk satu grup</span></span>
          </label>
        </div>
      </section>

      {error && <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
      {notice && <p style={{ color: "var(--color-emerald-800)", fontSize: 13 }}>{notice}</p>}
      <button type="button" onClick={() => void save()} disabled={saving} style={primaryBtn}>{saving ? "Menyimpan..." : "Simpan Pengaturan"}</button>
    </div>
  );
}

const wrap: React.CSSProperties = { display: "grid", gap: 16 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const label1: React.CSSProperties = { display: "flex", flexDirection: "column", gap: 6, fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff" };
const toggleRow: React.CSSProperties = { display: "flex", alignItems: "flex-start", gap: 10, fontSize: 13, padding: "8px 10px", background: "var(--color-cream-100)", borderRadius: 8 };
const primaryBtn: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, padding: "0 16px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, fontSize: 13, width: "fit-content" };
