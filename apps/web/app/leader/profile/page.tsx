"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { IconLogout, IconMail, IconPhone, IconUserCircle, IconUsersGroup } from "@tabler/icons-react";
import { authClient } from "@/lib/auth-client";
import { getMyAccessCached, invalidateMyAccessCache } from "@/lib/access-cache";
import AgentKycSelfSection from "@/components/agents/AgentKycSelfSection";

export default function LeaderProfilePage() {
  const router = useRouter();
  const { data: session } = authClient.useSession();
  const [phone, setPhone] = useState("");
  const [groups, setGroups] = useState<{ id: string; name: string }[]>([]);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [changing, setChanging] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    getMyAccessCached().then((access) => {
      setPhone(access.linkedAgent?.phone ?? "");
      setGroups(access.leaderGroups.map((g) => ({ id: g.id, name: g.name })));
    });
  }, []);

  async function submitChangePassword() {
    setError("");
    setNotice("");
    if (newPassword.length < 8) { setError("Kata sandi baru minimal 8 karakter."); return; }
    setChanging(true);
    try {
      await authClient.changePassword({ currentPassword, newPassword, revokeOtherSessions: true });
      setNotice("Kata sandi berhasil diubah.");
      setCurrentPassword("");
      setNewPassword("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengubah kata sandi.");
    } finally {
      setChanging(false);
    }
  }

  async function signOut() {
    await authClient.signOut();
    invalidateMyAccessCache();
    router.push("/sign-in");
  }

  return (
    <main style={page}>
      <p style={eyebrow}>PROFIL MUTTAWWIF</p>

      <section style={profileCard}>
        <div style={avatar}><IconUserCircle size={40} /></div>
        <h1 style={{ margin: "10px 0 2px", fontSize: 20 }}>{session?.user?.name || "Muttawwif"}</h1>
        <p style={{ margin: 0, display: "flex", alignItems: "center", gap: 6, opacity: .9, fontSize: 13 }}><IconMail size={14} />{session?.user?.email}</p>
        {phone && <p style={{ margin: "2px 0 0", display: "flex", alignItems: "center", gap: 6, opacity: .9, fontSize: 13 }}><IconPhone size={14} />{phone}</p>}
      </section>

      {groups.length > 0 && <section style={{ marginTop: 16 }}>
        <h2 style={sectionTitle}>GRUP YANG DIPIMPIN</h2>
        <div style={{ display: "grid", gap: 8 }}>
          {groups.map((g) => <div key={g.id} style={groupRow}><IconUsersGroup size={16} color="var(--color-emerald-800)" />{g.name}</div>)}
        </div>
      </section>}

      <AgentKycSelfSection />

      <section style={{ marginTop: 20 }}>
        <h2 style={sectionTitle}>GANTI KATA SANDI</h2>
        <div style={formCard}>
          <label style={{ display: "grid", gap: 6 }}>
            <span style={lab}>Kata sandi saat ini</span>
            <input type="password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} style={input} />
          </label>
          <label style={{ display: "grid", gap: 6, marginTop: 12 }}>
            <span style={lab}>Kata sandi baru</span>
            <input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} style={input} />
          </label>
          {error && <p style={errBox}>{error}</p>}
          {notice && <p style={successBox}>{notice}</p>}
          <button onClick={() => void submitChangePassword()} disabled={changing || !currentPassword || !newPassword} style={primary}>
            {changing ? "Menyimpan..." : "Simpan Kata Sandi Baru"}
          </button>
        </div>
      </section>

      <button onClick={() => void signOut()} style={signOutBtn}><IconLogout size={18} />Keluar</button>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "20px 20px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 12px" };
const profileCard: React.CSSProperties = { padding: 22, borderRadius: 16, background: "linear-gradient(135deg,var(--color-emerald-900),var(--color-emerald-800))", color: "#fff" };
const avatar: React.CSSProperties = { width: 64, height: 64, borderRadius: "50%", background: "rgba(255,255,255,.15)", display: "grid", placeItems: "center" };
const sectionTitle: React.CSSProperties = { fontSize: 13, fontWeight: 700, letterSpacing: ".08em", color: "var(--color-warm-400)", margin: "0 0 10px" };
const groupRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "10px 12px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 10, fontSize: 14 };
const formCard: React.CSSProperties = { padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12 };
const lab: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 46, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const errBox: React.CSSProperties = { margin: "12px 0 0", padding: 10, borderRadius: 8, background: "#ffe4e6", color: "var(--color-danger-600)", fontSize: 13 };
const successBox: React.CSSProperties = { margin: "12px 0 0", padding: 10, borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 13 };
const primary: React.CSSProperties = { width: "100%", minHeight: 48, marginTop: 16, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const signOutBtn: React.CSSProperties = { width: "100%", minHeight: 48, marginTop: 20, border: "1px solid var(--color-danger-600)", borderRadius: 10, background: "transparent", color: "var(--color-danger-600)", fontWeight: 700, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8 };
