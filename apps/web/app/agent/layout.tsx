"use client";

import { useRouter } from "next/navigation";
import { IconLogout } from "@tabler/icons-react";
import { RequireAccess } from "@/components/auth/RequireAccess";
import { authClient } from "@/lib/auth-client";
import { invalidateMyAccessCache } from "@/lib/access-cache";

function AgentShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();

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
    </div>
  );
}

export default function AgentLayout({ children }: { children: React.ReactNode }) {
  return (
    <RequireAccess role="agent">
      <AgentShell>{children}</AgentShell>
    </RequireAccess>
  );
}

const shell: React.CSSProperties = { minHeight: "100dvh", background: "var(--color-cream-100)" };
const header: React.CSSProperties = { position: "sticky", top: 0, zIndex: 30, display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 20px", background: "#fff", borderBottom: "1px solid var(--color-cream-400)" };
const brand: React.CSSProperties = { fontFamily: "'Playfair Display',serif", fontWeight: 700, fontSize: 18, color: "var(--color-emerald-900)" };
const signOutButton: React.CSSProperties = { minHeight: 36, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 12px", background: "transparent", color: "var(--color-warm-500)", display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13, fontWeight: 600 };
const content: React.CSSProperties = { minHeight: "calc(100dvh - 61px)" };
