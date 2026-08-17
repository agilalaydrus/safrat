"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { IconClipboardCheck, IconLogout, IconMessageCircle, IconSos, IconUsersGroup } from "@tabler/icons-react";
import { RequireAccess } from "@/components/auth/RequireAccess";
import { LeaderGroupProvider, useLeaderGroup } from "@/lib/leader-context";
import { authClient } from "@/lib/auth-client";
import { invalidateMyAccessCache } from "@/lib/access-cache";
import { useLeaderActiveSOSCount, useLeaderChatUnread } from "@/lib/leader-notifications";

const TABS = [
  ["Daftar Jamaah", "/leader", IconUsersGroup],
  ["Check-In", "/leader/check-in", IconClipboardCheck],
  ["Chat", "/leader/chat", IconMessageCircle],
  ["SOS", "/leader/sos", IconSos],
] as const;

function LeaderShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { groups, selectedGroupId, setSelectedGroupId } = useLeaderGroup();
  const chatUnread = useLeaderChatUnread(selectedGroupId, pathname === "/leader/chat");
  const sosActive = useLeaderActiveSOSCount();

  async function signOut() {
    await authClient.signOut();
    invalidateMyAccessCache();
    router.push("/sign-in");
  }

  return (
    <div style={shell}>
      <header style={header}>
        <div style={headerTop}>
          <span style={brand}>Safrat</span>
          <button onClick={() => void signOut()} style={signOutButton} aria-label="Keluar">
            <IconLogout size={18} />Keluar
          </button>
        </div>
        {groups.length > 1 && (
          <select value={selectedGroupId} onChange={(event) => setSelectedGroupId(event.target.value)} style={groupSelect}>
            {groups.map((group) => <option key={group.id} value={group.id}>{group.name}</option>)}
          </select>
        )}
      </header>
      <div style={content}>{children}</div>
      <nav style={tabBar} aria-label="Navigasi rombongan">
        {TABS.map(([label, href, Icon]) => {
          const active = pathname === href;
          const badgeCount = label === "Chat" ? chatUnread : label === "SOS" ? sosActive : 0;
          return (
            <Link key={label} href={href} style={{ ...tab, ...(active ? activeTab : {}) }}>
              <span style={iconWrap}>
                <Icon size={22} stroke={active ? 2.2 : 1.8} />
                {badgeCount > 0 && <span style={badge}>{badgeCount > 9 ? "9+" : badgeCount}</span>}
              </span>
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
const header: React.CSSProperties = { position: "sticky", top: 0, zIndex: 30, display: "grid", gap: 10, padding: "12px 20px", background: "#fff", borderBottom: "1px solid var(--color-cream-400)" };
const headerTop: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center" };
const brand: React.CSSProperties = { fontFamily: "'Playfair Display',serif", fontWeight: 700, fontSize: 18, color: "var(--color-emerald-900)" };
const signOutButton: React.CSSProperties = { minHeight: 36, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 12px", background: "transparent", color: "var(--color-warm-500)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 600 };
const groupSelect: React.CSSProperties = { width: "100%", minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: "0 12px", background: "#fff", font: "inherit" };
const content: React.CSSProperties = { flex: 1, paddingBottom: 84, overflowY: "auto" };
const tabBar: React.CSSProperties = { position: "fixed", bottom: 0, insetInline: 0, display: "flex", background: "#fff", borderTop: "1px solid var(--color-cream-400)", boxShadow: "0 -2px 12px rgba(26,20,16,.06)", paddingBottom: "env(safe-area-inset-bottom)", zIndex: 40 };
const tab: React.CSSProperties = { flex: 1, minHeight: 64, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 4, color: "var(--color-warm-400)" };
const activeTab: React.CSSProperties = { color: "var(--color-emerald-900)" };
const tabLabel: React.CSSProperties = { fontSize: 11, fontWeight: 600 };
const iconWrap: React.CSSProperties = { position: "relative", display: "inline-flex" };
const badge: React.CSSProperties = { position: "absolute", top: -6, right: -10, minWidth: 16, height: 16, borderRadius: 8, background: "var(--color-danger-600)", color: "#fff", fontSize: 10, fontWeight: 700, display: "flex", alignItems: "center", justifyContent: "center", padding: "0 4px", lineHeight: 1 };
