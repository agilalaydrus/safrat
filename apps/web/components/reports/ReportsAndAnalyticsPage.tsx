"use client";

import { useState } from "react";
import { IconChartBar, IconFileAnalytics, IconFilter, IconReportMoney } from "@tabler/icons-react";
import AnalyticsDashboard from "@/components/analytics/AnalyticsDashboard";
import FunnelDashboard from "@/components/funnel/FunnelDashboard";
import ProfitLossDashboard from "@/components/profitloss/ProfitLossDashboard";
import ReportsDashboard from "./ReportsDashboard";

const TABS = [
  ["analytics", "Analitik", IconChartBar],
  ["funnel", "Corong Pengunjung", IconFilter],
  ["profitloss", "Laba Rugi", IconReportMoney],
  ["export", "Ekspor Laporan", IconFileAnalytics],
] as const;

export default function ReportsAndAnalyticsPage() {
  const [tab, setTab] = useState<(typeof TABS)[number][0]>("analytics");

  return (
    <div style={page}>
      <div style={tabBar}>
        {TABS.map(([id, label, Icon]) => (
          <button key={id} onClick={() => setTab(id)} style={tab === id ? tabActive : tabInactive}>
            <Icon size={16} />{label}
          </button>
        ))}
      </div>
      {tab === "analytics" && <AnalyticsDashboard />}
      {tab === "funnel" && <FunnelDashboard />}
      {tab === "profitloss" && <ProfitLossDashboard />}
      {tab === "export" && <ReportsDashboard />}
    </div>
  );
}

const page: React.CSSProperties = {};
const tabBar: React.CSSProperties = { display: "flex", gap: 8, margin: "32px 24px 0", borderBottom: "1px solid var(--color-cream-400)" };
const tabBase: React.CSSProperties = { minHeight: 44, display: "inline-flex", alignItems: "center", gap: 6, padding: "0 16px", border: 0, borderBottomWidth: 2, borderBottomStyle: "solid", borderBottomColor: "transparent", background: "transparent", fontWeight: 700, fontSize: 14, color: "var(--color-warm-500)" };
const tabActive: React.CSSProperties = { ...tabBase, color: "var(--color-emerald-900)", borderBottomColor: "var(--color-gold-500)" };
const tabInactive: React.CSSProperties = tabBase;
