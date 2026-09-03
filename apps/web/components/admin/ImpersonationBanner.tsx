"use client";

import { useCallback, useEffect, useState } from "react";
import { IconEyeglass, IconX } from "@tabler/icons-react";
import { clearImpersonation, currentImpersonation, type ImpersonationState } from "@/lib/impersonation";
import { platformClient } from "@/lib/rpc";

/**
 * Shown on every screen while an impersonation session is open.
 *
 * It is deliberately loud and impossible to dismiss without ending the
 * session. Somebody who forgets which account they are looking at will
 * eventually tell a customer something about a different customer, and the
 * screen itself is the only thing that can prevent that.
 */
export default function ImpersonationBanner() {
  const [state, setState] = useState<ImpersonationState>();
  const [remaining, setRemaining] = useState("");
  const [ending, setEnding] = useState(false);

  useEffect(() => {
    setState(currentImpersonation());
  }, []);

  useEffect(() => {
    if (!state) return;
    const tick = () => {
      const left = state.expiresAt - Date.now();
      if (left <= 0) {
        // The server has already stopped honouring it; clearing here keeps the
        // browser from sending a dead token on every call.
        clearImpersonation();
        setState(undefined);
        window.location.reload();
        return;
      }
      const minutes = Math.floor(left / 60_000);
      const seconds = Math.floor((left % 60_000) / 1000);
      setRemaining(`${minutes}:${String(seconds).padStart(2, "0")}`);
    };
    tick();
    const timer = window.setInterval(tick, 1000);
    return () => window.clearInterval(timer);
  }, [state]);

  const end = useCallback(async () => {
    if (!state) return;
    setEnding(true);
    try {
      await platformClient.endImpersonation({ token: state.token });
    } catch {
      // Ending server-side may fail — the network is down, the session already
      // expired. The local token is dropped either way: leaving it behind
      // would keep the browser impersonating a session nobody can close.
    }
    clearImpersonation();
    window.location.href = "/admin";
  }, [state]);

  if (!state) return null;

  return (
    <div style={bar} role="alert">
      <IconEyeglass size={18} />
      <span style={{ fontWeight: 800 }}>Mode lihat-saja</span>
      <span style={{ opacity: 0.92 }}>
        Anda sedang melihat <strong>{state.operatorName}</strong>. Tidak ada perubahan yang bisa disimpan dari sini.
      </span>
      <span style={clock}>berakhir dalam {remaining}</span>
      <button type="button" onClick={end} disabled={ending} style={endButton}>
        <IconX size={14} />{ending ? "Menutup…" : "Akhiri"}
      </button>
    </div>
  );
}

const bar: React.CSSProperties = {
  position: "sticky", top: 0, zIndex: 60, display: "flex", alignItems: "center", gap: 12,
  flexWrap: "wrap", padding: "10px 20px", background: "var(--color-warning-700)", color: "#fff",
  fontSize: 13, lineHeight: 1.5,
};
const clock: React.CSSProperties = { marginLeft: "auto", fontVariantNumeric: "tabular-nums", fontWeight: 700, opacity: 0.95 };
const endButton: React.CSSProperties = {
  display: "inline-flex", alignItems: "center", gap: 5, minHeight: 32, padding: "0 14px",
  borderRadius: 7, border: "1px solid rgba(255,255,255,.55)", background: "transparent",
  color: "#fff", font: "inherit", fontWeight: 700, fontSize: 12, cursor: "pointer",
};
