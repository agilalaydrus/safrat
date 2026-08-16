"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { IconGenderFemale, IconGenderMale, IconPlane, IconWheelchair, IconWifiOff } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { groupLeaderClient, kloterClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";

export default function LeaderRosterPage() {
  const { groupId } = useParams<{ groupId: string }>();
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [kloterCodes, setKloterCodes] = useState<Record<string, string>>({});
  const [fromCache, setFromCache] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    cachedFetch(`leader-roster:${groupId}`, () => groupLeaderClient.getGroupRoster({ groupId }))
      .then((result) => {
        if (result.data) setPilgrims(result.data.pilgrims);
        setFromCache(result.fromCache);
      })
      .catch(() => setError("Gagal memuat daftar jamaah rombongan ini."));
  }, [groupId]);

  useEffect(() => {
    groupLeaderClient.listMyGroups({}).then((response) => {
      const seasonId = response.groups.find((g) => g.id === groupId)?.seasonId;
      if (!seasonId) return;
      kloterClient.listKloters({ seasonId }).then((r) => setKloterCodes(Object.fromEntries(r.kloters.map((k) => [k.id, k.code])))).catch(() => {});
    }).catch(() => {});
  }, [groupId]);

  return (
    <main style={page}>
      <p style={eyebrow}>DAFTAR JAMAAH</p>
      <h1 style={title}>{pilgrims.length} jamaah</h1>
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Menampilkan data tersimpan — Anda sedang offline</p>}
      {error && <p style={{ color: "var(--color-danger-600)" }}>{error}</p>}
      <div style={list}>
        {pilgrims.map((pilgrim) => (
          <article key={pilgrim.id} style={card}>
            <div>
              <strong>{pilgrim.fullName}</strong>
              <p style={meta}>{pilgrim.gender === Gender.FEMALE ? <IconGenderFemale size={15} /> : <IconGenderMale size={15} />}{pilgrim.passportNumber}{pilgrim.kloterId && kloterCodes[pilgrim.kloterId] && <span style={kloterMeta}><IconPlane size={13} />{kloterCodes[pilgrim.kloterId]}</span>}</p>
            </div>
            {pilgrim.requiresWheelchair && <IconWheelchair size={20} color="var(--color-gold-800)" />}
          </article>
        ))}
      </div>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "20px 20px 0" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 26, margin: "0 0 16px" };
const offlineBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, background: "var(--color-gold-50)", color: "var(--color-gold-800)", padding: "8px 12px", borderRadius: 8, fontSize: 12, marginBottom: 16 };
const list: React.CSSProperties = { display: "grid", gap: 10 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14, display: "flex", justifyContent: "space-between", alignItems: "center" };
const meta: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, margin: "4px 0 0", color: "var(--color-warm-500)", fontSize: 12, fontFamily: "ui-monospace, monospace" };
const kloterMeta: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 3, marginInlineStart: 6, padding: "2px 6px", borderRadius: 99, background: "var(--color-gold-50)", color: "var(--color-gold-800)", fontFamily: "inherit", fontWeight: 700 };
