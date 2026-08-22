"use client";
import { FormEvent, useEffect, useRef, useState } from "react";
import { IconSend, IconX } from "@tabler/icons-react";
import { ChatMessage } from "@hajj-saas/proto-gen/hajj/v1/chat_pb";
import { chatClient } from "@/lib/rpc";

type P = { open: boolean; groupId: string; groupName: string; onClose: () => void };

export default function GroupChatPanel({ open, groupId, groupName, onClose }: P) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [notice, setNotice] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);

  const refresh = () => { if (groupId) chatClient.listGroupMessages({ groupId }).then((r) => setMessages(r.messages)).catch(() => setNotice("Tidak dapat memuat pesan.")); };

  useEffect(() => {
    if (!open || !groupId) return;
    setNotice(""); void refresh();
    const interval = window.setInterval(refresh, 8000);
    return () => window.clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, groupId]);

  useEffect(() => { bottomRef.current?.scrollIntoView({ block: "end" }); }, [messages.length]);

  if (!open) return null;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = draft.trim();
    if (!body) return;
    setDraft(""); setSending(true);
    try { await chatClient.sendGroupMessage({ groupId, body }); refresh(); }
    catch { setNotice("Pesan gagal terkirim."); }
    finally { setSending(false); }
  }

  return (
    <div role="dialog" aria-modal="true" aria-label={`Chat grup ${groupName}`} style={overlay}>
      <aside style={sheet}>
        <header style={header}>
          <div><p style={eyebrow}>PANTAU OBROLAN</p><h2 style={{ margin: 0 }}>{groupName}</h2></div>
          <button onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </header>
        <div className="gold-divider" />
        {notice && <p style={{ color: "var(--color-danger-600)", padding: "0 4px" }}>{notice}</p>}
        <div style={list}>
          {!messages.length && <p style={{ color: "var(--color-warm-400)", textAlign: "center", marginTop: 40 }}>Belum ada pesan di grup ini.</p>}
          {messages.map((message) => (
            <div key={message.id} style={{ ...bubble, ...(message.fromPilgrim ? theirs : mine) }}>
              <p style={sender}>{message.fromPilgrim ? message.senderName : `${message.senderName} (Admin/Ketua)`}</p>
              <p style={{ margin: 0 }}>{message.body}</p>
            </div>
          ))}
          <div ref={bottomRef} />
        </div>
        <form onSubmit={submit} style={composer}>
          <input value={draft} onChange={(event) => setDraft(event.target.value)} placeholder="Kirim pesan ke grup ini..." style={input} />
          <button disabled={sending || !draft.trim()} style={sendButton} aria-label="Kirim"><IconSend size={18} /></button>
        </form>
      </aside>
    </div>
  );
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)", backdropFilter: "blur(2px)" };
const sheet: React.CSSProperties = { width: "min(480px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "var(--color-cream-100)", padding: 24, boxShadow: "-6px 0 32px rgba(15,23,42,.12)" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const closeBtn: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)", flexShrink: 0 };
const list: React.CSSProperties = { flex: 1, overflowY: "auto", display: "flex", flexDirection: "column", gap: 8, padding: "12px 4px" };
const bubble: React.CSSProperties = { maxWidth: "85%", padding: "10px 14px", borderRadius: 14, fontSize: 14 };
const mine: React.CSSProperties = { alignSelf: "flex-end", background: "var(--color-emerald-900)", color: "#fff", borderBottomRightRadius: 4 };
const theirs: React.CSSProperties = { alignSelf: "flex-start", background: "#fff", border: "1px solid var(--color-cream-400)", borderBottomLeftRadius: 4 };
const sender: React.CSSProperties = { margin: "0 0 2px", fontSize: 11, fontWeight: 700, opacity: 0.75 };
const composer: React.CSSProperties = { display: "flex", gap: 8, paddingTop: 12, borderTop: "1px solid var(--color-cream-300)" };
const input: React.CSSProperties = { flex: 1, minHeight: 46, border: "1px solid var(--color-cream-400)", borderRadius: 23, padding: "0 16px", font: "inherit", background: "#fff" };
const sendButton: React.CSSProperties = { width: 46, height: 46, borderRadius: "50%", border: 0, background: "var(--color-emerald-900)", color: "#fff", display: "grid", placeItems: "center", flexShrink: 0 };
