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
    const key = READ_KEY_PREFIX + groupId;

    function markCaughtUp() {
      return chatClient.listGroupMessages({ groupId }).then((response) => {
        const last = response.messages[response.messages.length - 1];
        if (last) window.localStorage.setItem(key, last.id);
      }).catch(() => {});
    }

    if (viewingChat) {
      // /leader/chat itself already polls this exact RPC every 5s while
      // open — don't run a second independent poll alongside it. Unread is
      // definitionally 0 while looking at the thread; just catch the "last
      // read" marker up once now and once more on the way out, so
      // navigating away doesn't re-surface messages already seen.
      setUnread(0);
      void markCaughtUp();
      return () => { void markCaughtUp(); };
    }

    function poll() {
      chatClient.listGroupMessages({ groupId }).then((response) => {
        if (cancelled) return;
        const messages = response.messages;
        const lastReadId = window.localStorage.getItem(key);
        const lastReadIndex = lastReadId ? messages.findIndex((m) => m.id === lastReadId) : -1;
        if (lastReadIndex === -1) {
          // No usable marker on this device — see usePilgrimChatUnread for
          // why this must baseline to "caught up" instead of backfilling
          // the group's whole message history as unread.
          const last = messages[messages.length - 1];
          if (last) window.localStorage.setItem(key, last.id);
          setUnread(0);
          return;
        }
        setUnread(messages.slice(lastReadIndex + 1).filter((m) => m.fromPilgrim).length);
      }).catch(() => {});
    }

    poll();
    const interval = window.setInterval(poll, 10000);
    return () => { cancelled = true; window.clearInterval(interval); };
  }, [groupId, viewingChat]);

  return unread;
}

/**
 * Count of the leader's own SOS alerts still needing attention (not
 * RESOLVED). `paused` should be true while /leader/sos itself is open — that
 * page already polls this exact RPC every 10s for the full list, so this
 * badge just holds its last value rather than running a second identical
 * poll; it resumes live updates the moment the leader navigates away.
 */
export function useLeaderActiveSOSCount(paused: boolean): number {
  const [count, setCount] = useState(0);

  useEffect(() => {
    if (paused) return;
    let cancelled = false;
    function poll() {
      groupLeaderClient.listMySOSAlerts({}).then((response) => {
        if (!cancelled) setCount(response.alerts.filter((alert) => alert.status !== "RESOLVED").length);
      }).catch(() => {});
    }
    poll();
    const interval = window.setInterval(poll, 10000);
    return () => { cancelled = true; window.clearInterval(interval); };
  }, [paused]);

  return count;
}
