"use client";

import { useEffect, useState } from "react";
import { IconGenderFemale, IconGenderMale, IconPlane, IconUsersGroup, IconWheelchair, IconWifiOff } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { groupLeaderClient, kloterClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";
import { useLeaderGroup } from "@/lib/leader-context";

export default function LeaderRosterPage() {
  const { groups, loaded, selectedGroupId } = useLeaderGroup();
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [kloterCodes, setKloterCodes] = useState<Record<string, string>>({});
  const [fromCache, setFromCache] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!selectedGroupId) return;
    cachedFetch(`leader-roster:${selectedGroupId}`, () => groupLeaderClient.getGroupRoster({ groupId: selectedGroupId }))
      .then((result) => {
        if (result.data) setPilgrims(result.data.pilgrims);
        setFromCache(result.fromCache);
      })
      .catch(() => setError("Gagal memuat daftar jamaah rombongan ini."));
  }, [selectedGroupId]);

  useEffect(() => {
    if (!selectedGroupId) return;
    const seasonId = groups.find((g) => g.id === selectedGroupId)?.seasonId;
    if (!seasonId) return;
    kloterClient.listKloters({ seasonId }).then((r) => setKloterCodes(Object.fromEntries(r.kloters.map((k) => [k.id, k.code])))).catch(() => {});
  }, [selectedGroupId, groups]);

  if (loaded && !groups.length) {
    return (
      <main style={page}>
        <section style={empty}>
          <IconUsersGroup size={44} color="var(--color-warm-400)" />
          <p style={{ color: "var(--color-warm-500)" }}>Anda belum ditugaskan sebagai ketua rombongan mana pun. Hubungi operator Anda untuk penugasan.</p>
        </section>
      </main>
    );
  }

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
const title: React.CSSProperties = { fontSize: 26, margin: "0 0 14px" };
const offlineBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, background: "var(--color-gold-50)", color: "var(--color-gold-800)", padding: "8px 12px", borderRadius: 8, fontSize: 12, marginBottom: 14 };
const list: React.CSSProperties = { display: "grid", gap: 10 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14, display: "flex", justifyContent: "space-between", alignItems: "center" };
const meta: React.CSSProperties = { margin: "4px 0 0", display: "flex", alignItems: "center", gap: 6, color: "var(--color-warm-500)", fontSize: 12 };
const kloterMeta: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 3, marginLeft: 8 };
const empty: React.CSSProperties = { minHeight: 300, display: "grid", placeItems: "center", alignContent: "center", gap: 10, textAlign: "center", padding: 20 };
