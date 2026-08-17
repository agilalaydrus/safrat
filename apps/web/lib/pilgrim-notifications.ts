"use client";

import { useEffect, useState } from "react";
import { chatClient } from "./rpc";

const READ_KEY_PREFIX = "safrat:pilgrim-chat-read:";

/**
 * Unread-count badge for the pilgrim's Chat tab. There's no server-side
 * read state — like the offline cache elsewhere in this app, "read" is
 * just the id of the last message this device has seen, kept in
 * localStorage. While the chat page itself is open, unread is pinned to 0
 * and the marker is kept current so navigating away and back doesn't
 * re-count messages already on screen.
 */
export function usePilgrimChatUnread(code: string, viewingChat: boolean): number {
  const [unread, setUnread] = useState(0);

  useEffect(() => {
    if (!code) { setUnread(0); return; }
    let cancelled = false;

    function poll() {
      chatClient.listMyMessages({ appAccessCode: code }).then((response) => {
        if (cancelled) return;
        const messages = response.messages;
        const key = READ_KEY_PREFIX + code;
        if (viewingChat) {
          const last = messages[messages.length - 1];
          if (last) window.localStorage.setItem(key, last.id);
          setUnread(0);
          return;
        }
        const lastReadId = window.localStorage.getItem(key);
        const lastReadIndex = lastReadId ? messages.findIndex((m) => m.id === lastReadId) : -1;
        setUnread(messages.slice(lastReadIndex + 1).filter((m) => !m.fromPilgrim).length);
      }).catch(() => {});
    }

    poll();
    const interval = window.setInterval(poll, 10000);
    return () => { cancelled = true; window.clearInterval(interval); };
  }, [code, viewingChat]);

  return unread;
}
