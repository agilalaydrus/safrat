"use client";

import { useEffect, useState } from "react";
import { IconCopy, IconCheck } from "@tabler/icons-react";
import { getMyAccessCached } from "@/lib/access-cache";
import { seasonClient } from "@/lib/rpc";
import { buildTenantLink } from "@/lib/tenant-link";

export default function AgentReferralTab() {
  const [referralCode, setReferralCode] = useState("");
  const [operatorSlug, setOperatorSlug] = useState("");
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean; slug: string }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    getMyAccessCached().then((access) => {
      setReferralCode(access.linkedAgent?.referralCode ?? "");
      setOperatorSlug(access.operatorSlug ?? "");
    });
    seasonClient.listSeasons({}).then((r) => {
      setSeasons(r.seasons);
      setSeasonId(r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? "");
    }).catch(() => {});
  }, []);

  const selectedSeasonSlug = seasons.find((s) => s.id === seasonId)?.slug;
  const refLink = operatorSlug && selectedSeasonSlug ? `${buildTenantLink(operatorSlug, `/register/${selectedSeasonSlug}`)}?ref=${referralCode}` : "";

  const copy = async () => {
    if (!refLink) return;
    await navigator.clipboard.writeText(refLink);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div style={{ maxWidth: 560 }}>
      <div style={{ background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 28, marginBottom: 20 }}>
        <h3 style={{ margin: "0 0 4px", fontSize: 15, fontWeight: 700 }}>Kode Referral Anda</h3>
        <p style={{ color: "var(--color-warm-500)", fontSize: 13, margin: "0 0 16px" }}>
          Bagikan link di bawah ke calon jamaah. Setiap pendaftaran melalui link ini otomatis tercatat sebagai referral Anda.
        </p>
        <div style={{ background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "12px 16px", marginBottom: 16, fontFamily: "monospace", fontSize: 18, fontWeight: 700, color: "var(--color-emerald-900)", letterSpacing: ".1em" }}>
          {referralCode || "-"}
        </div>

        {seasons.length > 1 && (
          <label style={{ display: "block", marginBottom: 12 }}>
            <span style={{ display: "block", fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)", marginBottom: 6 }}>Musim tujuan pendaftaran</span>
            <select value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={{ minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 12px", background: "#fff", font: "inherit" }}>
              {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>)}
            </select>
          </label>
        )}

        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <div style={{ flex: 1, background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "10px 14px", fontSize: 12, color: "var(--color-warm-600)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {refLink || "Memuat..."}
          </div>
          <button onClick={copy} disabled={!refLink} style={{ height: 44, padding: "0 16px", background: copied ? "var(--color-emerald-700)" : "var(--color-emerald-900)", color: "#fff", border: "none", borderRadius: 8, fontWeight: 700, cursor: "pointer", display: "flex", alignItems: "center", gap: 6, whiteSpace: "nowrap" }}>
            {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
            {copied ? "Tersalin!" : "Salin Link"}
          </button>
        </div>
      </div>

      <div style={{ background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 10, padding: "16px 20px", fontSize: 13, color: "var(--color-warm-600)" }}>
        <p style={{ margin: "0 0 8px", fontWeight: 700 }}>Cara kerja referral:</p>
        <ol style={{ margin: 0, paddingLeft: 20, lineHeight: 1.8 }}>
          <li>Calon jamaah buka link referral Anda</li>
          <li>Isi formulir pendaftaran (nama, paspor, kontak)</li>
          <li>Operator meninjau pendaftaran dan menyimpannya sebagai referral Anda</li>
          <li>Komisi dihitung otomatis saat jamaah melakukan pembayaran</li>
        </ol>
      </div>
    </div>
  );
}
