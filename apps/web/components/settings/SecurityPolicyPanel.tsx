"use client";

import { useEffect, useState } from "react";
import { IconDeviceDesktop, IconShieldCheck, IconShieldLock, IconTrash } from "@tabler/icons-react";
import type { ActiveSession } from "@hajj-saas/proto-gen/hajj/v1/security_settings_pb";
import { securitySettingsClient } from "@/lib/rpc";

export default function SecurityPolicyPanel() {
  const [enabled, setEnabled] = useState(false);
  const [cidrText, setCidrText] = useState("");
  const [yourIp, setYourIp] = useState("");
  const [sessions, setSessions] = useState<ActiveSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  const refresh = () => {
    setLoading(true);
    Promise.all([
      securitySettingsClient.getSecurityPosture({}),
      securitySettingsClient.listActiveSessions({}),
    ]).then(([posture, sessionList]) => {
      setEnabled(posture.ipAllowlistEnabled);
      setCidrText(posture.ipAllowlistCidrs.join("\n"));
      setYourIp(posture.yourIp);
      setSessions(sessionList.sessions);
    }).catch(() => setError("Gagal memuat kebijakan keamanan.")).finally(() => setLoading(false));
  };
  useEffect(refresh, []);

  const save = async (nextEnabled: boolean) => {
    setError(""); setNotice(""); setSaving(true);
    const cidrs = cidrText.split("\n").map((l) => l.trim()).filter(Boolean);
    try {
      const posture = await securitySettingsClient.setIpAllowlist({ enabled: nextEnabled, cidrs });
      setEnabled(posture.ipAllowlistEnabled);
      setCidrText(posture.ipAllowlistCidrs.join("\n"));
      setNotice(nextEnabled ? "Pembatasan IP diaktifkan." : "Pembatasan IP dinonaktifkan.");
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menyimpan pembatasan IP.");
    } finally {
      setSaving(false);
    }
  };

  const revoke = async (session: ActiveSession) => {
    if (!window.confirm(session.isCurrentSession ? "Ini sesi Anda saat ini — keluar sekarang?" : `Keluarkan sesi ${session.userName}?`)) return;
    try {
      await securitySettingsClient.revokeSession({ sessionId: session.id });
      refresh();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal mencabut sesi.");
    }
  };

  return (
    <div style={wrap}>
      <section style={card}>
        <h2 style={{ margin: "0 0 4px", display: "flex", alignItems: "center", gap: 8 }}><IconShieldCheck size={20} />Jaminan yang Sudah Berlaku</h2>
        <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-warm-500)" }}>Berlaku untuk seluruh akun, tidak bisa dimatikan dari sini — ditulis di kode, bukan sekadar klaim di layar.</p>
        <ul style={claimList}>
          <li><strong>Verifikasi dua langkah wajib</strong> untuk setiap staf sebelum membuka dashboard.</li>
          <li><strong>Satu sesi aktif per akun</strong> — masuk di perangkat baru langsung mengeluarkan sesi lain milik akun yang sama, tanpa jeda.</li>
          <li><strong>Jejak audit tidak bisa diubah atau dihapus</strong> oleh aplikasi — peran aplikasi produksi tidak memiliki hak UPDATE/DELETE atas tabelnya.</li>
        </ul>
      </section>

      <section style={card}>
        <h2 style={{ margin: "0 0 4px", display: "flex", alignItems: "center", gap: 8 }}><IconShieldLock size={20} />Pembatasan IP</h2>
        <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-warm-500)" }}>
          Opsional. Saat diaktifkan, hanya alamat IP pada daftar yang bisa masuk ke dashboard ini. IP Anda saat ini: <strong>{yourIp || "..."}</strong>
        </p>
        {loading ? <p style={{ color: "var(--color-warm-400)" }}>Memuat...</p> : <>
          <textarea
            value={cidrText}
            onChange={(e) => setCidrText(e.target.value)}
            placeholder={"Satu rentang CIDR per baris, contoh:\n203.0.113.0/24\n198.51.100.42/32"}
            rows={4}
            style={textarea}
          />
          {error && <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
          {notice && <p style={{ color: "var(--color-emerald-800)", fontSize: 13 }}>{notice}</p>}
          <div style={{ display: "flex", gap: 8, marginTop: 8 }}>
            {enabled ? (
              <button type="button" onClick={() => void save(false)} disabled={saving} style={dangerBtn}>Nonaktifkan Pembatasan</button>
            ) : (
              <button type="button" onClick={() => void save(true)} disabled={saving} style={primaryBtn}>
                {saving ? "Menyimpan..." : "Aktifkan Pembatasan"}
              </button>
            )}
          </div>
          <p style={{ margin: "8px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>
            Tidak akan tersimpan jika IP Anda sendiri tidak termasuk dalam daftar — mencegah Anda mengunci akun Anda sendiri.
          </p>
        </>}
      </section>

      <section style={card}>
        <h2 style={{ margin: "0 0 12px", display: "flex", alignItems: "center", gap: 8 }}><IconDeviceDesktop size={20} />Sesi Aktif ({sessions.length})</h2>
        {sessions.length ? (
          <div style={{ display: "grid", gap: 8 }}>
            {sessions.map((s) => (
              <div key={s.id} style={sessionRow}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <strong>{s.userName}</strong>
                    {s.isCurrentSession && <span style={miniBadge}>Sesi ini</span>}
                  </div>
                  <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>
                    {s.userEmail} · {s.ipAddress || "IP tidak tercatat"} · {s.createdAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}
                  </p>
                  {s.userAgent && <p style={{ margin: "2px 0 0", fontSize: 11, color: "var(--color-warm-400)" }}>{s.userAgent}</p>}
                </div>
                <button type="button" onClick={() => void revoke(s)} style={iconBtnDanger} aria-label="Cabut sesi"><IconTrash size={14} /> Cabut</button>
              </div>
            ))}
          </div>
        ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Tidak ada sesi aktif.</p>}
      </section>
    </div>
  );
}

const wrap: React.CSSProperties = { display: "grid", gap: 16 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const claimList: React.CSSProperties = { margin: 0, paddingLeft: 20, display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const textarea: React.CSSProperties = { width: "100%", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: 10, font: "inherit", fontFamily: "ui-monospace,monospace", fontSize: 13, resize: "vertical" };
const primaryBtn: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, padding: "0 16px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, fontSize: 13 };
const dangerBtn: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-danger-600)", borderRadius: 8, padding: "0 16px", background: "#fff", color: "var(--color-danger-600)", fontWeight: 700, fontSize: 13 };
const sessionRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "10px 12px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 8, flexWrap: "wrap" };
const miniBadge: React.CSSProperties = { padding: "2px 6px", borderRadius: 99, background: "var(--color-emerald-100)", color: "var(--color-emerald-800)", fontSize: 10, fontWeight: 700 };
const iconBtnDanger: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 6, background: "#fff", color: "var(--color-danger-600)", fontSize: 12, padding: "0 10px", display: "inline-flex", alignItems: "center", gap: 4 };
