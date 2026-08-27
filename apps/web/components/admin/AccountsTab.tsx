"use client";

import { useCallback, useEffect, useState } from "react";
import { IconSearch, IconShieldCheck, IconShieldOff, IconLogout } from "@tabler/icons-react";
import { PlatformAccount } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

export default function AccountsTab() {
  const [accounts, setAccounts] = useState<PlatformAccount[]>([]);
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    platformClient.listAccounts({ search: query, limit: 50 })
      .then((r) => setAccounts(r.accounts))
      .catch(() => setError("Gagal memuat akun."))
      .finally(() => setLoading(false));
  }, [query]);
  useEffect(() => { load(); }, [load]);

  const act = async (userId: string, action: () => Promise<void>, done: string) => {
    setBusy(userId);
    setError("");
    setNotice("");
    try {
      await action();
      setNotice(done);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Tindakan gagal.");
    } finally {
      setBusy("");
    }
  };

  return (
    <section style={{ display: "grid", gap: 14 }}>
      <p style={muted}>
        Pencarian wajib diisi untuk daftar yang panjang — panel yang membalik-balik seluruh akun
        di semua travel adalah ekspor data yang menunggu terjadi.
      </p>

      <form
        onSubmit={(e) => { e.preventDefault(); setQuery(search.trim()); }}
        style={{ display: "flex", gap: 8, flexWrap: "wrap" }}
      >
        <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Cari nama atau email"
          style={{ ...input, minWidth: 240, flex: 1 }} />
        <button type="submit" style={primary}><IconSearch size={16} />Cari</button>
      </form>

      {notice && <p style={{ color: "var(--color-emerald-800)", margin: 0 }}>{notice}</p>}
      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      {loading ? <p style={muted}>Memuat...</p> : accounts.length === 0 ? (
        <p style={muted}>Tidak ada akun yang cocok.</p>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Akun", "Travel / Peran", "Keamanan", "Sesi", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {accounts.map((account) => (
                <tr key={account.userId} style={tr}>
                  <td style={td}>
                    <strong>{account.name || "(tanpa nama)"}</strong>
                    <small style={{ display: "block", color: "var(--color-warm-400)" }}>{account.email}</small>
                  </td>
                  <td style={td}>
                    {account.operatorName || <span style={{ color: "var(--color-warm-400)" }}>—</span>}
                    {account.orgRole && <small style={{ display: "block", color: "var(--color-warm-400)" }}>{account.orgRole}</small>}
                  </td>
                  <td style={td}>
                    <div style={{ display: "grid", gap: 4 }}>
                      {account.isPlatformAdmin && <span style={badgeAdmin}><IconShieldCheck size={12} />Admin platform</span>}
                      <span style={account.twoFactorEnabled ? badgeOk : badgeWarn}>
                        {account.twoFactorEnabled ? "2FA aktif" : "2FA belum aktif"}
                      </span>
                      {!account.emailVerified && <span style={badgeWarn}>Email belum diverifikasi</span>}
                    </div>
                  </td>
                  <td style={td}>{account.activeSessions}</td>
                  <td style={{ ...td, whiteSpace: "nowrap" }}>
                    <div style={{ display: "grid", gap: 6 }}>
                      {account.isPlatformAdmin ? (
                        <button style={ghost} disabled={busy === account.userId}
                          onClick={() => act(account.userId,
                            async () => { await platformClient.revokePlatformAdmin({ userId: account.userId }); },
                            "Akses admin platform dicabut.")}>
                          <IconShieldOff size={14} />Cabut admin
                        </button>
                      ) : (
                        <button style={ghost} disabled={busy === account.userId}
                          onClick={() => act(account.userId,
                            async () => { await platformClient.grantPlatformAdmin({ userId: account.userId, note: "" }); },
                            "Akses admin platform diberikan.")}>
                          <IconShieldCheck size={14} />Jadikan admin
                        </button>
                      )}
                      {/* The response to a suspected takeover: a password reset
                          changes nothing for whoever already holds a session. */}
                      {account.activeSessions > 0 && (
                        <button style={{ ...ghost, borderColor: "var(--color-danger-600)", color: "var(--color-danger-600)" }}
                          disabled={busy === account.userId}
                          onClick={() => act(account.userId,
                            async () => {
                              const result = await platformClient.revokeSessions({ userId: account.userId });
                              setNotice(`${result.sessionsEnded} sesi diakhiri.`);
                            },
                            "Sesi diakhiri.")}>
                          <IconLogout size={14} />Akhiri sesi
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 13, margin: 0 };
const input: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", fontSize: 14 };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 18px", background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 7 };
const ghost: React.CSSProperties = { minHeight: 36, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "transparent", color: "var(--color-warm-700)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 12, justifyContent: "center" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
const badgeBase: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, padding: "2px 8px", borderRadius: 99, fontSize: 11, fontWeight: 700, justifySelf: "start" };
const badgeAdmin: React.CSSProperties = { ...badgeBase, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)" };
const badgeOk: React.CSSProperties = { ...badgeBase, background: "var(--color-cream-200)", color: "var(--color-warm-700)" };
const badgeWarn: React.CSSProperties = { ...badgeBase, background: "var(--color-gold-50)", color: "#b45309" };
