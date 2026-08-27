"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { IconHome, IconSos, IconMessageCircle, IconCalendarEvent, IconShoppingBag, IconReceipt, IconLogout, IconSpeakerphone, IconMoonStars, IconDots, IconX } from "@tabler/icons-react";
import { useRegisterShellServiceWorker } from "@/lib/register-sw";
import { useLocationPing } from "@/lib/geolocation";
import { useRegisterPilgrimPush } from "@/lib/pilgrim-push";
import { RequireAccess } from "@/components/auth/RequireAccess";
import { RequireTwoFactor } from "@/components/auth/RequireTwoFactor";
import { PilgrimCodeProvider, usePilgrimCode } from "@/lib/pilgrim-context";
import { authClient } from "@/lib/auth-client";
import { invalidateMyAccessCache } from "@/lib/access-cache";
import { usePilgrimChatUnread } from "@/lib/pilgrim-notifications";
import { LostReportButton } from "@/components/pilgrim/LostReportButton";

// Four items in the bar — the ones a jamaah checks daily during the trip
// (home, schedule, SOS stays here since it's safety-critical, chat).
// Everything else is one tap away behind "Lainnya".
const PRIMARY_TABS = [
  ["Beranda", "/pilgrim", IconHome],
  ["Jadwal", "/pilgrim/schedule", IconCalendarEvent],
  ["SOS", "/pilgrim/sos", IconSos],
  ["Chat", "/pilgrim/chat", IconMessageCircle],
] as const;

const MORE_ITEMS = [
  ["Ibadah", "/pilgrim/rituals", IconMoonStars],
  ["Produk", "/pilgrim/products", IconShoppingBag],
  ["Transaksi", "/pilgrim/transactions", IconReceipt],
  ["Info", "/pilgrim/announcements", IconSpeakerphone],
] as const;

function PilgrimShell({ children }: { children: React.ReactNode }) {
  useRegisterShellServiceWorker();
  const pathname = usePathname();
  const router = useRouter();
  const code = usePilgrimCode();
  useLocationPing(code);
  useRegisterPilgrimPush(code);
  const chatUnread = usePilgrimChatUnread(code, pathname === "/pilgrim/chat");
  const [moreOpen, setMoreOpen] = useState(false);
  const moreActive = MORE_ITEMS.some(([, href]) => href === pathname);

  async function signOut() {
    await authClient.signOut();
    invalidateMyAccessCache();
    router.push("/sign-in");
  }

  return (
    <div style={shell}>
      <header style={header}>
        <span style={brand}>Tawafiq Hub</span>
        <button onClick={() => void signOut()} style={signOutButton} aria-label="Keluar">
          <IconLogout size={18} />Keluar
        </button>
      </header>
      <div style={content}>{children}</div>
      {code && <LostReportButton appAccessCode={code} />}
      <nav style={tabBar} aria-label="Navigasi aplikasi Jamaah">
        {PRIMARY_TABS.map(([label, href, Icon]) => {
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
        <button onClick={() => setMoreOpen(true)} style={{ ...tab, ...(moreActive ? activeTab : {}), border: 0, background: "transparent", font: "inherit" }} aria-label="Lainnya">
          <span style={iconWrap}><IconDots size={24} stroke={moreActive ? 2.2 : 1.8} /></span>
          <span style={tabLabel}>Lainnya</span>
        </button>
      </nav>

      {moreOpen && (
        <div style={sheetOverlay} onClick={() => setMoreOpen(false)}>
          <style>{"@keyframes pilgrim-sheet-up{from{transform:translateY(100%)}to{transform:translateY(0)}}"}</style>
          <div style={{ ...sheet, animation: "pilgrim-sheet-up .2s cubic-bezier(0,0,.2,1)" }} onClick={(e) => e.stopPropagation()}>
            <div style={sheetHeader}>
              <span style={sheetTitle}>Lainnya</span>
              <button onClick={() => setMoreOpen(false)} style={sheetClose} aria-label="Tutup"><IconX size={20} /></button>
            </div>
            <div style={sheetGrid}>
              {MORE_ITEMS.map(([label, href, Icon]) => {
                const active = pathname === href;
                return (
                  <Link key={label} href={href} onClick={() => setMoreOpen(false)} style={{ ...sheetItem, ...(active ? sheetItemActive : {}) }}>
                    <span style={iconWrap}><Icon size={24} stroke={active ? 2.2 : 1.8} /></span>
                    <span style={sheetItemLabel}>{label}</span>
                  </Link>
                );
              })}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default function PilgrimLayout({ children }: { children: React.ReactNode }) {
  return (
    <RequireAccess role="pilgrim">
      {/* prompt, not enforce: SOS lives behind this shell, and standing between
          a jamaah in distress and the button that summons help is a hazard, not
          a security improvement. */}
      <RequireTwoFactor mode="prompt">
      <PilgrimCodeProvider>
        <PilgrimShell>{children}</PilgrimShell>
      </PilgrimCodeProvider>
      </RequireTwoFactor>
    </RequireAccess>
  );
}

const shell: React.CSSProperties = { minHeight: "100dvh", display: "flex", flexDirection: "column", background: "var(--color-cream-100)" };
const header: React.CSSProperties = { position: "sticky", top: 0, zIndex: 30, display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 20px", background: "#fff", borderBottom: "1px solid var(--color-cream-400)" };
const brand: React.CSSProperties = { fontFamily: "'Playfair Display',serif", fontWeight: 700, fontSize: 18, color: "var(--color-emerald-900)" };
const signOutButton: React.CSSProperties = { minHeight: 36, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 12px", background: "transparent", color: "var(--color-warm-500)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 600 };
const content: React.CSSProperties = { flex: 1, paddingBottom: 84, overflowY: "auto" };
const tabBar: React.CSSProperties = { position: "fixed", bottom: 0, insetInline: 0, display: "flex", background: "#fff", borderTop: "1px solid var(--color-cream-400)", boxShadow: "0 -2px 12px rgba(15,23,42,.06)", paddingBottom: "env(safe-area-inset-bottom)", zIndex: 40 };
const tab: React.CSSProperties = { flex: 1, minHeight: 64, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 4, color: "var(--color-warm-400)" };
const activeTab: React.CSSProperties = { color: "var(--color-emerald-900)" };
const tabLabel: React.CSSProperties = { fontSize: 11, fontWeight: 600 };
const iconWrap: React.CSSProperties = { position: "relative", display: "inline-flex" };
const badge: React.CSSProperties = { position: "absolute", top: -6, right: -10, minWidth: 16, height: 16, borderRadius: 8, background: "var(--color-danger-600)", color: "#fff", fontSize: 10, fontWeight: 700, display: "flex", alignItems: "center", justifyContent: "center", padding: "0 4px", lineHeight: 1 };
const sheetOverlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 50, background: "rgba(15,23,42,.44)", display: "flex", alignItems: "flex-end" };
const sheet: React.CSSProperties = { width: "100%", background: "#fff", borderRadius: "18px 18px 0 0", padding: "8px 20px calc(20px + env(safe-area-inset-bottom))" };
const sheetHeader: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 0 6px" };
const sheetTitle: React.CSSProperties = { fontFamily: "'Playfair Display',serif", fontWeight: 700, fontSize: 18, color: "var(--color-emerald-900)" };
const sheetClose: React.CSSProperties = { width: 36, height: 36, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-500)", display: "flex", alignItems: "center", justifyContent: "center" };
const sheetGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 8, padding: "8px 0 4px" };
const sheetItem: React.CSSProperties = { display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 8, minHeight: 84, borderRadius: 14, background: "var(--color-cream-100)", color: "var(--color-warm-600)", textDecoration: "none" };
const sheetItemActive: React.CSSProperties = { background: "var(--color-emerald-50)", color: "var(--color-emerald-900)" };
const sheetItemLabel: React.CSSProperties = { fontSize: 12, fontWeight: 600, textAlign: "center" };
