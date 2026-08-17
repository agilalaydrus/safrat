"use client";

import { useEffect, useState } from "react";
import { chatClient, groupLeaderClient } from "./rpc";

const READ_KEY_PREFIX = "safrat:leader-chat-read:";

/** Same localStorage-marker approach as usePilgrimChatUnread, keyed per group. */
export function useLeaderChatUnread(groupId: string, viewingChat: boolean): number {
  const [unread, setUnread] = useState(0);

  useEffect(() => {
    if (!groupId) { setUnread(0); return; }
    let cancelled = false;

    function poll() {
      chatClient.listGroupMessages({ groupId }).then((response) => {
        if (cancelled) return;
        const messages = response.messages;
        const key = READ_KEY_PREFIX + groupId;
        if (viewingChat) {
          const last = messages[messages.length - 1];
          if (last) window.localStorage.setItem(key, last.id);
          setUnread(0);
          return;
        }
        const lastReadId = window.localStorage.getItem(key);
        const lastReadIndex = lastReadId ? messages.findIndex((m) => m.id === lastReadId) : -1;
        setUnread(messages.slice(lastReadIndex + 1).filter((m) => m.fromPilgrim).length);
      }).catch(() => {});
    }

    poll();
    const interval = window.setInterval(poll, 10000);
    return () => { cancelled = true; window.clearInterval(interval); };
  }, [groupId, viewingChat]);

  return unread;
}

/** Count of the leader's own SOS alerts still needing attention (not RESOLVED). */
export function useLeaderActiveSOSCount(): number {
  const [count, setCount] = useState(0);

  useEffect(() => {
    let cancelled = false;
    function poll() {
      groupLeaderClient.listMySOSAlerts({}).then((response) => {
        if (!cancelled) setCount(response.alerts.filter((alert) => alert.status !== "RESOLVED").length);
      }).catch(() => {});
    }
    poll();
    const interval = window.setInterval(poll, 10000);
    return () => { cancelled = true; window.clearInterval(interval); };
  }, []);

  return count;
}
