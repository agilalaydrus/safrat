"use client";

import { useEffect, useState } from "react";
import { IconWallet, IconUsers, IconLink, IconClock, IconMapPin, IconShieldCheck } from "@tabler/icons-react";
import AgentWalletTab from "@/components/agent/AgentWalletTab";
import AgentJamaahTab from "@/components/agent/AgentJamaahTab";
import AgentReferralTab from "@/components/agent/AgentReferralTab";
import AgentPayoutTab from "@/components/agent/AgentPayoutTab";
import AgentTripTab from "@/components/agent/AgentTripTab";
import AgentKycSelfSection from "@/components/agents/AgentKycSelfSection";
import { authClient } from "@/lib/auth-client";
import { staffScheduleClient } from "@/lib/rpc";

const BASE_TABS = [
  { id: "wallet", label: "Dompet Komisi", icon: IconWallet },
  { id: "jamaah", label: "Jamaah Saya", icon: IconUsers },
  { id: "referral", label: "Link Referral", icon: IconLink },
  { id: "payout", label: "Pencairan", icon: IconClock },
  { id: "kyc", label: "KYC", icon: IconShieldCheck },
] as const;
const TRIP_TAB = { id: "trip", label: "Perjalanan Saya", icon: IconMapPin } as const;
type TabId = (typeof BASE_TABS)[number]["id"] | typeof TRIP_TAB.id;

export default function AgentPortalPage() {
  const [tab, setTab] = useState<TabId>("wallet");
  const [hasTrip, setHasTrip] = useState(false);
  const { data: session } = authClient.useSession();

  useEffect(() => {
    staffScheduleClient.listMyAssignments({}).then((r) => setHasTrip(r.assignments.length > 0)).catch(() => {});
  }, []);

  const TABS = hasTrip ? [...BASE_TABS, TRIP_TAB] : BASE_TABS;

  return (
    <main style={page}>
      <p style={eyebrow}>TOUR LEADER PORTAL</p>
      <h1 style={title}>Selamat datang, {session?.user?.name?.split(" ")[0]}</h1>
      <p style={{ color: "var(--color-warm-500)", margin: "0 0 20px" }}>Kelola komisi, jamaah referral, dan pencairan dana Anda.</p>
      <div className="gold-divider" />
      <div style={tabBar}>
        {TABS.map((t) => (
          <button key={t.id} onClick={() => setTab(t.id)} style={tab === t.id ? tabActive : tabInactive}>
            <t.icon size={17} />{t.label}
          </button>
        ))}
      </div>
      {tab === "wallet" && <AgentWalletTab />}
      {tab === "jamaah" && <AgentJamaahTab />}
      {tab === "referral" && <AgentReferralTab />}
      {tab === "payout" && <AgentPayoutTab />}
      {tab === "trip" && <AgentTripTab />}
      {tab === "kyc" && <AgentKycSelfSection />}
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 860, margin: "0 auto", padding: "32px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,44px)", fontWeight: 500, margin: "0 0 4px" };
const tabBar: React.CSSProperties = { display: "flex", gap: 8, margin: "16px 0 28px", flexWrap: "wrap" };
const tabActive: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "#fff", fontWeight: 700, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center" };
const tabInactive: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "var(--color-cream-200)", color: "var(--color-warm-700)", fontWeight: 600, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center" };
