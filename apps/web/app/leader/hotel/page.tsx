"use client";

import { useEffect, useState } from "react";
import { IconCheck, IconWifiOff } from "@tabler/icons-react";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { groupLeaderClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";
import { useLeaderGroup } from "@/lib/leader-context";

export default function LeaderHotelCheckInPage() {
  const { selectedGroupId: groupId } = useLeaderGroup();
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [fromCache, setFromCache] = useState(false);
  const [working, setWorking] = useState("");
  const [notice, setNotice] = useState("");

  const refresh = () => {
    if (!groupId) return;
    cachedFetch(`leader-roster:${groupId}`, () => groupLeaderClient.getGroupRoster({ groupId })).then((result) => {
      if (result.data) setPilgrims(result.data.pilgrims);
      setFromCache(result.fromCache);
    });
  };
  useEffect(refresh, [groupId]);

  async function toggle(pilgrim: Pilgrim) {
    setWorking(pilgrim.id);
    setNotice("");
    try {
      const result = await groupLeaderClient.checkInGroupPilgrimHotel({ pilgrimId: pilgrim.id, checkedIn: !pilgrim.hotelCheckedIn });
      setPilgrims((current) => current.map((p) => (p.id === result.id ? result : p)));
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal memperbarui status check-in hotel.");
    } finally {
      setWorking("");
    }
  }

  const checkedInCount = pilgrims.filter((p) => p.hotelCheckedIn).length;

  return (
    <main style={page}>
      <p style={eyebrow}>CHECK-IN HOTEL</p>
      <p style={summary}>{checkedInCount} dari {pilgrims.length} jamaah sudah check-in</p>
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Menggunakan data tersimpan — Anda sedang offline</p>}
      {notice && <p style={noticeStyle}>{notice}</p>}
      <div style={list}>
        {pilgrims.map((pilgrim) => (
          <article key={pilgrim.id} style={card}>
            <div>
              <strong style={{ display: "block" }}>{pilgrim.fullName}</strong>
              <span style={{ fontSize: 12, color: "var(--color-warm-400)" }}>{pilgrim.passportNumber}</span>
            </div>
            <button disabled={working === pilgrim.id} onClick={() => toggle(pilgrim)} style={pilgrim.hotelCheckedIn ? doneButton : actionButton}>
              {pilgrim.hotelCheckedIn && <IconCheck size={16} />}{pilgrim.hotelCheckedIn ? "Sudah Check-in" : "Tandai Check-in"}
            </button>
          </article>
        ))}
        {!pilgrims.length && <p style={{ color: "var(--color-warm-400)" }}>Belum ada jamaah di grup ini.</p>}
      </div>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "20px 20px 0" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const summary: React.CSSProperties = { margin: "0 0 12px", color: "var(--color-warm-500)" };
const offlineBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, background: "var(--color-gold-50)", color: "var(--color-gold-800)", padding: "8px 12px", borderRadius: 8, fontSize: 12, marginBottom: 14 };
const noticeStyle: React.CSSProperties = { color: "var(--color-danger-600)", fontSize: 13, marginBottom: 12 };
const list: React.CSSProperties = { display: "grid", gap: 10 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 14, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12 };
const actionButton: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-emerald-800)", borderRadius: 8, background: "transparent", color: "var(--color-emerald-900)", fontWeight: 600, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 6, padding: "0 14px", flexShrink: 0 };
const doneButton: React.CSSProperties = { ...actionButton, border: "1px solid var(--color-cream-400)", background: "var(--color-emerald-50)", color: "var(--color-emerald-900)" };
