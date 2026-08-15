"use client";

import Link from "next/link";
import { usePathname, useParams } from "next/navigation";
import { IconHome, IconSos, IconMessageCircle, IconCalendarEvent, IconShoppingBag } from "@tabler/icons-react";
import { useRegisterShellServiceWorker } from "@/lib/register-sw";
import { useLocationPing } from "@/lib/geolocation";

const TABS = [
  ["Beranda", "", IconHome],
  ["SOS", "sos", IconSos],
  ["Chat", "chat", IconMessageCircle],
  ["Jadwal", "schedule", IconCalendarEvent],
  ["Produk", "products", IconShoppingBag],
] as const;

export default function PilgrimLayout({ children }: { children: React.ReactNode }) {
  useRegisterShellServiceWorker();
  const pathname = usePathname();
  const { code } = useParams<{ code: string }>();
  const base = `/pilgrim/${code}`;
  useLocationPing(code);

  return (
    <div style={shell}>
      <div style={content}>{children}</div>
      <nav style={tabBar} aria-label="Navigasi aplikasi Jamaah">
        {TABS.map(([label, segment, Icon]) => {
          const href = segment ? `${base}/${segment}` : base;
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

const shell: React.CSSProperties = { minHeight: "100dvh", display: "flex", flexDirection: "column", background: "var(--color-cream-100)" };
const content: React.CSSProperties = { flex: 1, paddingBottom: 84, overflowY: "auto" };
const tabBar: React.CSSProperties = { position: "fixed", bottom: 0, insetInline: 0, display: "flex", background: "#fff", borderTop: "1px solid var(--color-cream-400)", boxShadow: "0 -2px 12px rgba(26,20,16,.06)", paddingBottom: "env(safe-area-inset-bottom)", zIndex: 40 };
const tab: React.CSSProperties = { flex: 1, minHeight: 64, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 4, color: "var(--color-warm-400)" };
const activeTab: React.CSSProperties = { color: "var(--color-emerald-900)" };
const tabLabel: React.CSSProperties = { fontSize: 11, fontWeight: 600 };
