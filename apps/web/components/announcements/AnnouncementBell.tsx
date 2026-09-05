"use client";

import { useEffect, useRef, useState } from "react";
import { IconBell } from "@tabler/icons-react";
import type { OperatorAnnouncement } from "@hajj-saas/proto-gen/hajj/v1/announcement_pb";
import { announcementClient } from "@/lib/rpc";

const dateTime = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "";

/**
 * Pengumuman (E2, TUGAS-PANEL-SAAS.md §10.1 DESAIN) — the in-app channel that
 * didn't exist before this: everything the platform actually sent to this
 * tenant, read independently of every other tenant. Fetched once on mount
 * and again each time the panel opens, since a new announcement can arrive
 * at any moment and this is the one place a staff member would notice.
 */
export function AnnouncementBell() {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<OperatorAnnouncement[]>([]);
  const [loaded, setLoaded] = useState(false);
  // Marked-read locally, ahead of the next refetch confirming it server-side
  // — an id in here reads as read immediately without needing to fabricate a
  // fake Timestamp for the real readAt field.
  const [locallyRead, setLocallyRead] = useState<Set<string>>(new Set());
  const containerRef = useRef<HTMLDivElement>(null);

  const load = () => {
    announcementClient.listMyAnnouncements({}).then((r) => setItems(r.announcements)).catch(() => {}).finally(() => setLoaded(true));
  };

  useEffect(() => { load(); }, []);

  useEffect(() => {
    if (!open) return;
    load();
    const onClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onClickOutside);
    return () => document.removeEventListener("mousedown", onClickOutside);
  }, [open]);

  const isRead = (a: OperatorAnnouncement) => Boolean(a.readAt) || locallyRead.has(a.id);
  const unreadCount = items.filter((a) => !isRead(a)).length;

  const markRead = (id: string) => {
    if (locallyRead.has(id) && items.find((a) => a.id === id)?.readAt) return;
    setLocallyRead((prev) => new Set(prev).add(id));
    announcementClient.markAnnouncementRead({ announcementId: id }).catch(() => {
      setLocallyRead((prev) => { const next = new Set(prev); next.delete(id); return next; });
    });
  };

  return (
    <div ref={containerRef} style={{ position: "relative" }}>
      <button
        type="button"
        className="dashboard-bell-button"
        aria-label={unreadCount > 0 ? `${unreadCount} pengumuman belum dibaca` : "Pengumuman"}
        onClick={() => setOpen((v) => !v)}
        style={{ position: "relative" }}
      >
        <IconBell size={20} stroke={1.8} />
        {unreadCount > 0 && <span style={badge}>{unreadCount > 9 ? "9+" : unreadCount}</span>}
      </button>

      {open && (
        <div style={panel} role="dialog" aria-label="Pengumuman dari TawafiqHub">
          <div style={panelHead}>
            <strong style={{ fontSize: 13 }}>Pengumuman</strong>
          </div>
          <div style={panelBody}>
            {!loaded && <p className="tw-note" style={{ padding: 14 }}>Memuat…</p>}
            {loaded && items.length === 0 && (
              <p className="tw-note" style={{ padding: 14 }}>Belum ada pengumuman dari TawafiqHub.</p>
            )}
            {items.map((a) => (
              <button key={a.id} type="button" onClick={() => markRead(a.id)} style={{ ...item, ...(isRead(a) ? {} : itemUnread) }}>
                <div style={{ display: "flex", justifyContent: "space-between", gap: 8 }}>
                  <strong style={{ fontSize: 13 }}>{a.title}</strong>
                  {!isRead(a) && <span style={dot} aria-hidden />}
                </div>
                <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--color-warm-600)", whiteSpace: "pre-wrap" }}>{a.body}</p>
                {a.link && <span style={{ fontSize: 12, color: "var(--color-emerald-800)" }}>{a.link}</span>}
                <span style={{ display: "block", marginTop: 6, fontSize: 11, color: "var(--color-warm-400)" }}>{dateTime(a.sentAt)}</span>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

const badge: React.CSSProperties = {
  position: "absolute", top: -4, right: -4, minWidth: 16, height: 16, padding: "0 4px",
  borderRadius: 999, background: "var(--color-danger-600)", color: "#fff", fontSize: 10, fontWeight: 800,
  display: "grid", placeItems: "center", lineHeight: 1,
};
const panel: React.CSSProperties = {
  position: "absolute", top: "calc(100% + 8px)", right: 0, width: "min(360px, 90vw)", maxHeight: 420,
  display: "flex", flexDirection: "column", borderRadius: 12, border: "1px solid var(--dashboard-border)",
  background: "#fff", boxShadow: "var(--shadow-lift)", zIndex: 50, overflow: "hidden",
};
const panelHead: React.CSSProperties = { padding: "10px 14px", borderBottom: "1px solid var(--color-cream-300)" };
const panelBody: React.CSSProperties = { overflowY: "auto" };
const item: React.CSSProperties = {
  display: "block", width: "100%", textAlign: "left", padding: "12px 14px", border: 0,
  borderBottom: "1px solid var(--color-cream-200)", background: "#fff", cursor: "pointer", font: "inherit",
};
const itemUnread: React.CSSProperties = { background: "var(--color-emerald-50)" };
const dot: React.CSSProperties = { width: 8, height: 8, borderRadius: "50%", background: "var(--color-emerald-700)", flexShrink: 0, marginTop: 4 };
