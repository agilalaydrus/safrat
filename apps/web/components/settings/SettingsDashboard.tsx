"use client";

import { useState } from "react";
import { IconBuilding, IconUsersGroup, IconWorld } from "@tabler/icons-react";
import OperatorProfilePanel from "./OperatorProfilePanel";
import TeamPanel from "./TeamPanel";
import DomainPanel from "./DomainPanel";

export default function SettingsDashboard() {
  const [tab, setTab] = useState<"profil" | "tim" | "domain">("profil");

  return <main style={page}>
    <header><p style={eyebrow}>PENGATURAN</p><h1 style={title}>Pengaturan</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>3 area pengaturan · {tab === "profil" ? "Profil Operator" : tab === "tim" ? "Tim & Anggota" : "Domain"} sedang dibuka</p></header>
    <div className="gold-divider" />
    <div style={tabBar}>
      <button onClick={() => setTab("profil")} style={tab === "profil" ? tabActive : tabInactive}><IconBuilding size={18} />Profil Operator</button>
      <button onClick={() => setTab("tim")} style={tab === "tim" ? tabActive : tabInactive}><IconUsersGroup size={18} />Tim &amp; Anggota</button>
      <button onClick={() => setTab("domain")} style={tab === "domain" ? tabActive : tabInactive}><IconWorld size={18} />Domain</button>
    </div>
    {tab === "profil" && <OperatorProfilePanel />}
    {tab === "tim" && <TeamPanel />}
    {tab === "domain" && <DomainPanel />}
  </main>;
}

const page: React.CSSProperties = { maxWidth: 800, margin: "0 auto", padding: "32px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const tabBar: React.CSSProperties = { display: "flex", gap: 8, margin: "16px 0 24px" };
const tabActive: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center" };
const tabInactive: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "var(--color-cream-200)", color: "var(--color-warm-700)", fontWeight: 600, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center" };
