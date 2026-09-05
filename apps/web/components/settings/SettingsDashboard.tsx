"use client";

import { useState } from "react";
import { IconBellRinging, IconBuilding, IconDownload, IconShieldLock, IconUsersGroup, IconWorld } from "@tabler/icons-react";
import OperatorProfilePanel from "./OperatorProfilePanel";
import TeamPanel from "./TeamPanel";
import DomainPanel from "./DomainPanel";
import DataExportPanel from "./DataExportPanel";
import SecurityPolicyPanel from "./SecurityPolicyPanel";
import NotificationSettingsPanel from "./NotificationSettingsPanel";

export default function SettingsDashboard() {
  const [tab, setTab] = useState<"profil" | "tim" | "domain" | "keamanan" | "notifikasi" | "data">("profil");

  const TAB_LABEL: Record<typeof tab, string> = { profil: "Profil Operator", tim: "Tim & Anggota", domain: "Domain", keamanan: "Kebijakan Keamanan", notifikasi: "Notifikasi", data: "Ekspor Data Saya" };

  return <main style={page}>
    <header><p style={eyebrow}>PENGATURAN</p><h1 style={title}>Pengaturan</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>6 area pengaturan · {TAB_LABEL[tab]} sedang dibuka</p></header>
    <div className="gold-divider" />
    <div style={tabBar}>
      <button onClick={() => setTab("profil")} style={tab === "profil" ? tabActive : tabInactive}><IconBuilding size={18} />Profil Operator</button>
      <button onClick={() => setTab("tim")} style={tab === "tim" ? tabActive : tabInactive}><IconUsersGroup size={18} />Tim &amp; Anggota</button>
      <button onClick={() => setTab("domain")} style={tab === "domain" ? tabActive : tabInactive}><IconWorld size={18} />Domain</button>
      <button onClick={() => setTab("keamanan")} style={tab === "keamanan" ? tabActive : tabInactive}><IconShieldLock size={18} />Kebijakan Keamanan</button>
      <button onClick={() => setTab("notifikasi")} style={tab === "notifikasi" ? tabActive : tabInactive}><IconBellRinging size={18} />Notifikasi</button>
      <button onClick={() => setTab("data")} style={tab === "data" ? tabActive : tabInactive}><IconDownload size={18} />Ekspor Data Saya</button>
    </div>
    {tab === "profil" && <OperatorProfilePanel />}
    {tab === "tim" && <TeamPanel />}
    {tab === "domain" && <DomainPanel />}
    {tab === "keamanan" && <SecurityPolicyPanel />}
    {tab === "notifikasi" && <NotificationSettingsPanel />}
    {tab === "data" && <DataExportPanel />}
  </main>;
}

const page: React.CSSProperties = { maxWidth: 800, margin: "0 auto", padding: "32px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const tabBar: React.CSSProperties = { display: "flex", gap: 8, margin: "16px 0 24px" };
const tabActive: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-gold-100)", borderRadius: 12, background: "var(--color-gold-50)", color: "var(--color-gold-600)", boxShadow: "0 0 0 4px color-mix(in srgb, var(--color-gold-500) 12%, transparent)", fontWeight: 700, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center" };
const tabInactive: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "var(--color-cream-200)", color: "var(--color-warm-700)", fontWeight: 600, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center" };
