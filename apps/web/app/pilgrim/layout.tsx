"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { IconHome, IconSos, IconMessageCircle, IconCalendarEvent, IconShoppingBag, IconLogout } from "@tabler/icons-react";
import { useRegisterShellServiceWorker } from "@/lib/register-sw";
import { useLocationPing } from "@/lib/geolocation";
import { RequireAccess } from "@/components/auth/RequireAccess";
import { PilgrimCodeProvider, usePilgrimCode } from "@/lib/pilgrim-context";
import { authClient } from "@/lib/auth-client";
import { invalidateMyAccessCache } from "@/lib/access-cache";
import { usePilgrimChatUnread } from "@/lib/pilgrim-notifications";

const TABS = [
  ["Beranda", "/pilgrim", IconHome],
  ["SOS", "/pilgrim/sos", IconSos],
  ["Chat", "/pilgrim/chat", IconMessageCircle],
  ["Jadwal", "/pilgrim/schedule", IconCalendarEvent],
  ["Produk", "/pilgrim/products", IconShoppingBag],
] as const;

function PilgrimShell({ children }: { children: React.ReactNode }) {
  useRegisterShellServiceWorker();
  const pathname = usePathname();
  const router = useRouter();
  const code = usePilgrimCode();
  useLocationPing(code);
  const chatUnread = usePilgrimChatUnread(code, pathname === "/pilgrim/chat");

  async function signOut() {
    await authClient.signOut();
    invalidateMyAccessCache();
    router.push("/sign-in");
  }

  return (
    <div style={shell}>
      <header style={header}>
        <span style={brand}>Safrat</span>
        <button onClick={() => void signOut()} style={signOutButton} aria-label="Keluar">
          <IconLogout size={18} />Keluar
        </button>
      </header>
      <div style={content}>{children}</div>
      <nav style={tabBar} aria-label="Navigasi aplikasi Jamaah">
        {TABS.map(([label, href, Icon]) => {
          const active = pathname === href;
          const badgeCount = label === "Chat" ? chatUnread : 0;
          return (
            <Link key={label} href={href} style={{ ...tab, ...(active ? activeTab : {}) }}>
              <span style={iconWrap}>
                <Icon size={24} stroke={active ? 2.2 : 1.8} />
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

export default function PilgrimLayout({ children }: { children: React.ReactNode }) {
  return (
    <RequireAccess role="pilgrim">
      <PilgrimCodeProvider>
        <PilgrimShell>{children}</PilgrimShell>
      </PilgrimCodeProvider>
    </RequireAccess>
  );
}

const shell: React.CSSProperties = { minHeight: "100dvh", display: "flex", flexDirection: "column", background: "var(--color-cream-100)" };
const header: React.CSSProperties = { position: "sticky", top: 0, zIndex: 30, display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 20px", background: "#fff", borderBottom: "1px solid var(--color-cream-400)" };
const brand: React.CSSProperties = { fontFamily: "'Playfair Display',serif", fontWeight: 700, fontSize: 18, color: "var(--color-emerald-900)" };
const signOutButton: React.CSSProperties = { minHeight: 36, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 12px", background: "transparent", color: "var(--color-warm-500)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 600 };
const content: React.CSSProperties = { flex: 1, paddingBottom: 84, overflowY: "auto" };
const tabBar: React.CSSProperties = { position: "fixed", bottom: 0, insetInline: 0, display: "flex", background: "#fff", borderTop: "1px solid var(--color-cream-400)", boxShadow: "0 -2px 12px rgba(26,20,16,.06)", paddingBottom: "env(safe-area-inset-bottom)", zIndex: 40 };
const tab: React.CSSProperties = { flex: 1, minHeight: 64, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 4, color: "var(--color-warm-400)" };
const activeTab: React.CSSProperties = { color: "var(--color-emerald-900)" };
const tabLabel: React.CSSProperties = { fontSize: 11, fontWeight: 600 };
const iconWrap: React.CSSProperties = { position: "relative", display: "inline-flex" };
const badge: React.CSSProperties = { position: "absolute", top: -6, right: -10, minWidth: 16, height: 16, borderRadius: 8, background: "var(--color-danger-600)", color: "#fff", fontSize: 10, fontWeight: 700, display: "flex", alignItems: "center", justifyContent: "center", padding: "0 4px", lineHeight: 1 };
