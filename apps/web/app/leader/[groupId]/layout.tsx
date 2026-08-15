"use client";

import Link from "next/link";
import { usePathname, useParams } from "next/navigation";
import { IconArrowLeft, IconClipboardCheck, IconMessageCircle, IconSos, IconUsersGroup } from "@tabler/icons-react";

const TABS = [
  ["Daftar Jamaah", "", IconUsersGroup],
  ["Check-In", "check-in", IconClipboardCheck],
  ["Chat", "chat", IconMessageCircle],
  ["SOS", "sos", IconSos],
] as const;

export default function LeaderGroupLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { groupId } = useParams<{ groupId: string }>();
  const base = `/leader/${groupId}`;

  return (
    <div style={shell}>
      <header style={header}><Link href="/leader" style={backLink}><IconArrowLeft size={18} />Semua rombongan</Link></header>
      <div style={content}>{children}</div>
      <nav style={tabBar} aria-label="Navigasi rombongan">
        {TABS.map(([label, segment, Icon]) => {
          const href = segment ? `${base}/${segment}` : base;
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

const shell: React.CSSProperties = { minHeight: "100dvh", display: "flex", flexDirection: "column", background: "var(--color-cream-100)" };
const header: React.CSSProperties = { padding: "16px 20px 0" };
const backLink: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, color: "var(--color-gold-800)", fontWeight: 600, fontSize: 13 };
const content: React.CSSProperties = { flex: 1, paddingBottom: 84, overflowY: "auto" };
const tabBar: React.CSSProperties = { position: "fixed", bottom: 0, insetInline: 0, display: "flex", background: "#fff", borderTop: "1px solid var(--color-cream-400)", boxShadow: "0 -2px 12px rgba(26,20,16,.06)", paddingBottom: "env(safe-area-inset-bottom)", zIndex: 40 };
const tab: React.CSSProperties = { flex: 1, minHeight: 64, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 4, color: "var(--color-warm-400)" };
const activeTab: React.CSSProperties = { color: "var(--color-emerald-900)" };
const tabLabel: React.CSSProperties = { fontSize: 11, fontWeight: 600 };
