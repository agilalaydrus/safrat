"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { IconSend, IconWifiOff } from "@tabler/icons-react";
import { ChatMessage } from "@hajj-saas/proto-gen/hajj/v1/chat_pb";
import { chatClient } from "@/lib/rpc";
import { cachedFetch, enqueueAction, useOfflineQueueFlush } from "@/lib/offline";

const QUEUE_KIND = "leader-chat-message";

export default function LeaderChatPage() {
  const { groupId } = useParams<{ groupId: string }>();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [offline, setOffline] = useState(false);
  const [sending, setSending] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  const refresh = () => {
    cachedFetch(`leader-chat:${groupId}`, () => chatClient.listGroupMessages({ groupId })).then((result) => {
      if (result.data) setMessages(result.data.messages);
      setOffline(result.fromCache);
    });
  };

  useEffect(() => {
    refresh();
    const interval = window.setInterval(refresh, 5000);
    return () => window.clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [groupId]);

  useOfflineQueueFlush(QUEUE_KIND, async (payload) => {
    const { body } = payload as { body: string };
    await chatClient.sendGroupMessage({ groupId, body });
    refresh();
  });

  useEffect(() => { bottomRef.current?.scrollIntoView({ block: "end" }); }, [messages.length]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = draft.trim();
    if (!body) return;
    setDraft("");
    setSending(true);
    try {
      await chatClient.sendGroupMessage({ groupId, body });
      refresh();
    } catch {
      enqueueAction(QUEUE_KIND, { body });
      setMessages((current) => [...current, new ChatMessage({ id: `local-${Date.now()}`, senderName: "You", fromPilgrim: false, body })]);
    } finally {
      setSending(false);
    }
  }

  return (
    <main style={page}>
      {offline && <p style={offlineBanner}><IconWifiOff size={16} />Showing saved messages — you're offline</p>}
      <div style={list}>
        {messages.length === 0 && <p style={{ color: "var(--color-warm-400)", textAlign: "center", marginTop: 40 }}>No messages yet.</p>}
        {messages.map((message) => (
          <div key={message.id} style={{ ...bubble, ...(message.fromPilgrim ? theirs : mine) }}>
            {message.fromPilgrim && <p style={sender}>{message.senderName}</p>}
            <p style={{ margin: 0 }}>{message.body}</p>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
      <form onSubmit={submit} style={composer}>
        <input value={draft} onChange={(event) => setDraft(event.target.value)} placeholder="Message this group..." style={input} />
        <button disabled={sending || !draft.trim()} style={sendButton} aria-label="Send"><IconSend size={20} /></button>
      </form>
    </main>
  );
}

const page: React.CSSProperties = { display: "flex", flexDirection: "column", height: "calc(100dvh - 140px)" };
const offlineBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, background: "var(--color-gold-50)", color: "var(--color-gold-800)", padding: "8px 16px", fontSize: 12 };
const list: React.CSSProperties = { flex: 1, overflowY: "auto", padding: "16px 16px 8px", display: "flex", flexDirection: "column", gap: 8 };
const bubble: React.CSSProperties = { maxWidth: "80%", padding: "10px 14px", borderRadius: 14, fontSize: 14 };
const mine: React.CSSProperties = { alignSelf: "flex-end", background: "var(--color-emerald-900)", color: "#fff", borderBottomRightRadius: 4 };
const theirs: React.CSSProperties = { alignSelf: "flex-start", background: "#fff", border: "1px solid var(--color-cream-400)", borderBottomLeftRadius: 4 };
const sender: React.CSSProperties = { margin: "0 0 2px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)" };
const composer: React.CSSProperties = { display: "flex", gap: 8, padding: 12, background: "#fff", borderTop: "1px solid var(--color-cream-400)" };
const input: React.CSSProperties = { flex: 1, minHeight: 46, border: "1px solid var(--color-cream-400)", borderRadius: 23, padding: "0 16px", font: "inherit" };
const sendButton: React.CSSProperties = { width: 46, height: 46, borderRadius: "50%", border: 0, background: "var(--color-emerald-900)", color: "#fff", display: "grid", placeItems: "center", flexShrink: 0 };
