"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { IconClipboardCheck, IconMessageCircle, IconSos, IconUsersGroup } from "@tabler/icons-react";
import { RequireAccess } from "@/components/auth/RequireAccess";
import { LeaderGroupProvider, useLeaderGroup } from "@/lib/leader-context";

const TABS = [
  ["Daftar Jamaah", "/leader", IconUsersGroup],
  ["Check-In", "/leader/check-in", IconClipboardCheck],
  ["Chat", "/leader/chat", IconMessageCircle],
  ["SOS", "/leader/sos", IconSos],
] as const;

function LeaderShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { groups, selectedGroupId, setSelectedGroupId } = useLeaderGroup();

  return (
    <div style={shell}>
      {groups.length > 1 && (
        <header style={header}>
          <select value={selectedGroupId} onChange={(event) => setSelectedGroupId(event.target.value)} style={groupSelect}>
            {groups.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}
          </select>
        </header>
      )}
      <div style={content}>{children}</div>
      <nav style={tabBar} aria-label="Navigasi rombongan">
        {TABS.map(([label, href, Icon]) => {
          const active = pathname === href;
          return (
            <Link key={label} href={href} style={{ ...tab, ...(active ? activeTab : {}) }}>
              <Icon size={22} stroke={active ? 2.2 : 1.8} />
              <span style={tabLabel}>{label}</span>
            </Link>
          );
        })}
      </nav>
    </div>
  );
}

export default function LeaderLayout({ children }: { children: React.ReactNode }) {
  return (
    <RequireAccess role="leader">
      <LeaderGroupProvider>
        <LeaderShell>{children}</LeaderShell>
      </LeaderGroupProvider>
    </RequireAccess>
  );
}

const shell: React.CSSProperties = { minHeight: "100dvh", display: "flex", flexDirection: "column", background: "var(--color-cream-100)" };
const header: React.CSSProperties = { padding: "12px 20px 0" };
const groupSelect: React.CSSProperties = { width: "100%", minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: "0 12px", background: "#fff", font: "inherit" };
const content: React.CSSProperties = { flex: 1, paddingBottom: 84, overflowY: "auto" };
const tabBar: React.CSSProperties = { position: "fixed", bottom: 0, insetInline: 0, display: "flex", background: "#fff", borderTop: "1px solid var(--color-cream-400)", boxShadow: "0 -2px 12px rgba(26,20,16,.06)", paddingBottom: "env(safe-area-inset-bottom)", zIndex: 40 };
const tab: React.CSSProperties = { flex: 1, minHeight: 64, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 4, color: "var(--color-warm-400)" };
const activeTab: React.CSSProperties = { color: "var(--color-emerald-900)" };
const tabLabel: React.CSSProperties = { fontSize: 11, fontWeight: 600 };
