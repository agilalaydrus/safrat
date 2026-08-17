"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { IconHome, IconSos, IconMessageCircle, IconCalendarEvent, IconShoppingBag } from "@tabler/icons-react";
import { useRegisterShellServiceWorker } from "@/lib/register-sw";
import { useLocationPing } from "@/lib/geolocation";
import { RequireAccess } from "@/components/auth/RequireAccess";
import { PilgrimCodeProvider, usePilgrimCode } from "@/lib/pilgrim-context";

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
  const code = usePilgrimCode();
  useLocationPing(code);

  return (
    <div style={shell}>
      <div style={content}>{children}</div>
      <nav style={tabBar} aria-label="Navigasi aplikasi Jamaah">
        {TABS.map(([label, href, Icon]) => {
          const active = pathname === href;
          return (
            <Link key={label} href={href} style={{ ...tab, ...(active ? activeTab : {}) }}>
              <Icon size={24} stroke={active ? 2.2 : 1.8} />
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
const content: React.CSSProperties = { flex: 1, paddingBottom: 84, overflowY: "auto" };
const tabBar: React.CSSProperties = { position: "fixed", bottom: 0, insetInline: 0, display: "flex", background: "#fff", borderTop: "1px solid var(--color-cream-400)", boxShadow: "0 -2px 12px rgba(26,20,16,.06)", paddingBottom: "env(safe-area-inset-bottom)", zIndex: 40 };
const tab: React.CSSProperties = { flex: 1, minHeight: 64, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 4, color: "var(--color-warm-400)" };
const activeTab: React.CSSProperties = { color: "var(--color-emerald-900)" };
const tabLabel: React.CSSProperties = { fontSize: 11, fontWeight: 600 };
