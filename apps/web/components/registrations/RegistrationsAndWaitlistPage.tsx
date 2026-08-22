"use client";

import { useState } from "react";
import { IconClipboardList, IconClock } from "@tabler/icons-react";
import RegistrationsDashboard from "./RegistrationsDashboard";
import WaitlistDashboard from "@/components/waitlist/WaitlistDashboard";

const TABS = [
  ["registrations", "Pendaftaran", IconClipboardList],
  ["waitlist", "Daftar Tunggu", IconClock],
] as const;

export default function RegistrationsAndWaitlistPage() {
  const [tab, setTab] = useState<(typeof TABS)[number][0]>("registrations");

  return (
    <div>
      <div style={tabBar}>
        {TABS.map(([id, label, Icon]) => (
          <button key={id} onClick={() => setTab(id)} style={tab === id ? tabActive : tabInactive}>
            <Icon size={16} />{label}
          </button>
        ))}
      </div>
      {tab === "registrations" ? <RegistrationsDashboard /> : <WaitlistDashboard />}
    </div>
  );
}

const tabBar: React.CSSProperties = { display: "flex", gap: 8, margin: "32px 24px 0", borderBottom: "1px solid var(--color-cream-400)" };
const tabBase: React.CSSProperties = { minHeight: 44, display: "inline-flex", alignItems: "center", gap: 6, padding: "0 16px", border: 0, borderBottomWidth: 2, borderBottomStyle: "solid", borderBottomColor: "transparent", background: "transparent", fontWeight: 700, fontSize: 14, color: "var(--color-warm-500)" };
const tabActive: React.CSSProperties = { ...tabBase, color: "var(--color-emerald-900)", borderBottomColor: "var(--color-gold-500)" };
const tabInactive: React.CSSProperties = tabBase;
