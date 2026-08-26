"use client";

import { useCallback, useEffect, useState } from "react";
import { ConnectError } from "@connectrpc/connect";
import { IconCheck, IconCopy, IconTrash, IconWorld } from "@tabler/icons-react";
import type { OperatorDomain } from "@hajj-saas/proto-gen/hajj/v1/operator_pb";
import { operatorClient } from "@/lib/rpc";

/**
 * Lets an operator point their own domain at their storefront.
 *
 * Adding a domain only *claims* it. Nothing is served on it, allowed through
 * CORS, or issued a certificate until the DNS TXT record proves control — so
 * the panel leads with the record rather than treating verification as an
 * afterthought.
 */
export default function DomainPanel() {
  const [domains, setDomains] = useState<OperatorDomain[]>([]);
  const [hostname, setHostname] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [copied, setCopied] = useState("");

  const load = useCallback(async () => {
    try {
      const response = await operatorClient.listMyDomains({});
      setDomains(response.domains);
    } catch (caught) {
      setError(ConnectError.from(caught).rawMessage);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await action();
      await load();
    } catch (caught) {
      setError(ConnectError.from(caught).rawMessage);
    } finally {
      setBusy(false);
    }
  };

  const add = () => run(async () => {
    const value = hostname.trim().toLowerCase().replace(/^https?:\/\//, "").replace(/\/.*$/, "");
    if (!value) throw new ConnectError("Isi nama domain terlebih dahulu.");
    await operatorClient.addMyDomain({ hostname: value });
    setHostname("");
    setNotice("Domain ditambahkan. Publikasikan data TXT di bawah, lalu klik Verifikasi.");
  });

  const verify = (domain: OperatorDomain) => run(async () => {
    await operatorClient.verifyMyDomain({ domainId: domain.id });
    setNotice(`${domain.hostname} terverifikasi.`);
  });

  const remove = (domain: OperatorDomain) => run(async () => {
    await operatorClient.removeMyDomain({ domainId: domain.id });
    setNotice(`${domain.hostname} dihapus.`);
  });

  const copy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(value);
      window.setTimeout(() => setCopied(""), 1800);
    } catch {
      // Clipboard access can be denied; the value stays selectable on screen.
    }
  };

  return <section style={card}>
    <header>
      <p style={eyebrow}>DOMAIN SENDIRI</p>
      <h2 style={{ margin: "6px 0 0", fontSize: 22 }}>Gunakan domain travel Anda</h2>
      <p style={muted}>Arahkan domain milik Anda ke halaman travel ini. Pengunjung akan melihat alamat Anda sendiri, bukan subdomain TawafiqHub.</p>
    </header>

    <div style={addRow}>
      <input
        value={hostname}
        onChange={(event) => setHostname(event.target.value)}
        onKeyDown={(event) => { if (event.key === "Enter") void add(); }}
        placeholder="umrohvacana.com"
        aria-label="Nama domain"
        spellCheck={false}
        style={input}
      />
      <button type="button" onClick={() => void add()} disabled={busy} style={primaryButton}>Tambah domain</button>
    </div>

    {error && <p style={alertError}>{error}</p>}
    {notice && <p style={alertOk}>{notice}</p>}

    {domains.length === 0
      ? <p style={muted}>Belum ada domain. Anda tetap bisa memakai subdomain TawafiqHub seperti biasa.</p>
      : <ul style={list}>
          {domains.map((domain) => <li key={domain.id} style={row}>
            <div style={{ display: "grid", gap: 6, minWidth: 0 }}>
              <strong style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <IconWorld size={17} />{domain.hostname}
                <span style={domain.verified ? badgeOk : badgePending}>{domain.verified ? "Terverifikasi" : "Menunggu verifikasi"}</span>
              </strong>
              {!domain.verified && <div style={{ display: "grid", gap: 6 }}>
                <span style={muted}>Tambahkan data DNS berikut di penyedia domain Anda, lalu klik Verifikasi. Propagasi DNS bisa memakan waktu beberapa jam.</span>
                <div style={recordGrid}>
                  <span style={recordLabel}>Tipe</span><code style={code}>TXT</code>
                  <span style={recordLabel}>Nama</span>
                  <code style={code}>{domain.verificationRecord}
                    <button type="button" onClick={() => void copy(domain.verificationRecord)} style={copyButton} aria-label="Salin nama data">
                      {copied === domain.verificationRecord ? <IconCheck size={14} /> : <IconCopy size={14} />}
                    </button>
                  </code>
                  <span style={recordLabel}>Nilai</span>
                  <code style={code}>{domain.verificationToken}
                    <button type="button" onClick={() => void copy(domain.verificationToken)} style={copyButton} aria-label="Salin nilai data">
                      {copied === domain.verificationToken ? <IconCheck size={14} /> : <IconCopy size={14} />}
                    </button>
                  </code>
                </div>
              </div>}
            </div>
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              {!domain.verified && <button type="button" onClick={() => void verify(domain)} disabled={busy} style={secondaryButton}>Verifikasi</button>}
              <button type="button" onClick={() => void remove(domain)} disabled={busy} style={iconButton} aria-label={`Hapus ${domain.hostname}`}><IconTrash size={16} /></button>
            </div>
          </li>)}
        </ul>}
  </section>;
}

const card: React.CSSProperties = { display: "grid", gap: 18, border: "1px solid var(--color-cream-400)", borderRadius: 14, background: "var(--color-cream-100)", padding: 24 };
const eyebrow: React.CSSProperties = { margin: 0, color: "var(--color-warm-400)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em" };
const muted: React.CSSProperties = { margin: 0, color: "var(--color-warm-500)", fontSize: 13, lineHeight: 1.6 };
const addRow: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap" };
const input: React.CSSProperties = { minHeight: 44, flex: "1 1 260px", border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: "0 12px", fontSize: 14 };
const primaryButton: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 10, background: "var(--color-emerald-900)", padding: "0 18px", color: "#fff", fontSize: 14, fontWeight: 800, cursor: "pointer" };
const secondaryButton: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 9, background: "#fff", padding: "0 14px", fontSize: 13, fontWeight: 800, cursor: "pointer" };
const iconButton: React.CSSProperties = { display: "grid", width: 38, height: 38, placeItems: "center", border: "1px solid var(--color-cream-400)", borderRadius: 9, background: "#fff", cursor: "pointer" };
const list: React.CSSProperties = { display: "grid", gap: 12, margin: 0, padding: 0, listStyle: "none" };
const row: React.CSSProperties = { display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16, flexWrap: "wrap", border: "1px solid var(--color-cream-400)", borderRadius: 12, background: "#fff", padding: 14 };
const badgeOk: React.CSSProperties = { borderRadius: 999, background: "var(--color-emerald-900)", padding: "3px 10px", color: "#fff", fontSize: 11, fontWeight: 800 };
// Explicit amber rather than a token: "waiting" must not read as "done" at a
// glance, which is exactly what happens when both badges resolve to green.
const badgePending: React.CSSProperties = { borderRadius: 999, background: "#b45309", padding: "3px 10px", color: "#fff", fontSize: 11, fontWeight: 800 };
const recordGrid: React.CSSProperties = { display: "grid", gap: "6px 12px", gridTemplateColumns: "auto 1fr", alignItems: "center" };
const recordLabel: React.CSSProperties = { color: "var(--color-warm-400)", fontSize: 11, fontWeight: 800, letterSpacing: "0.08em" };
const code: React.CSSProperties = { display: "flex", overflowWrap: "anywhere", alignItems: "center", justifyContent: "space-between", gap: 8, borderRadius: 8, background: "var(--color-cream-200)", padding: "6px 10px", fontFamily: "ui-monospace, monospace", fontSize: 12.5 };
const copyButton: React.CSSProperties = { display: "grid", flex: "none", placeItems: "center", border: 0, background: "transparent", color: "var(--color-warm-500)", cursor: "pointer" };
const alertError: React.CSSProperties = { margin: 0, border: "1px solid var(--color-gold-600)", borderRadius: 10, background: "#fff", padding: "10px 12px", fontSize: 13 };
const alertOk: React.CSSProperties = { margin: 0, border: "1px solid var(--color-emerald-900)", borderRadius: 10, background: "#fff", padding: "10px 12px", fontSize: 13 };
