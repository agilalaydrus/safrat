"use client";

import { useEffect } from "react";

// A simple client-side cache-and-queue for the Pilgrim/Leader PWAs — not a
// PowerSync-grade offline guarantee, just enough to keep the last-seen data
// visible and let write actions (SOS, check-ins) survive a dropped
// connection until the browser comes back online. See CLAUDE.md.

const CACHE_PREFIX = "safrat:cache:";
const QUEUE_KEY = "safrat:queue";

export function readCache<T>(key: string): T | undefined {
  if (typeof window === "undefined") return undefined;
  try {
    const raw = window.localStorage.getItem(CACHE_PREFIX + key);
    return raw ? (JSON.parse(raw) as T) : undefined;
  } catch {
    return undefined;
  }
}

export function writeCache<T>(key: string, value: T) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(CACHE_PREFIX + key, JSON.stringify(value));
  } catch {
    // storage full or unavailable — cache is best-effort, never throw
  }
}

/** Fetches fresh data, falling back to the last cached value on failure (e.g. offline). Caches successful results. */
export async function cachedFetch<T>(key: string, fetcher: () => Promise<T>): Promise<{ data: T | undefined; fromCache: boolean }> {
  try {
    const data = await fetcher();
    writeCache(key, data);
    return { data, fromCache: false };
  } catch (error) {
    const cached = readCache<T>(key);
    if (cached !== undefined) return { data: cached, fromCache: true };
    throw error;
  }
}

type QueuedAction = { id: string; kind: string; payload: unknown; queuedAt: string };

function readQueue(): QueuedAction[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(QUEUE_KEY);
    return raw ? (JSON.parse(raw) as QueuedAction[]) : [];
  } catch {
    return [];
  }
}

function writeQueue(queue: QueuedAction[]) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(QUEUE_KEY, JSON.stringify(queue));
}

/** Queues a write action for replay when back online. Returns the queued item's id. */
export function enqueueAction(kind: string, payload: unknown): string {
  const id = `${kind}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const queue = readQueue();
  queue.push({ id, kind, payload, queuedAt: new Date().toISOString() });
  writeQueue(queue);
  return id;
}

export function listQueuedActions(kind?: string): QueuedAction[] {
  const queue = readQueue();
  return kind ? queue.filter((item) => item.kind === kind) : queue;
}

function removeFromQueue(id: string) {
  writeQueue(readQueue().filter((item) => item.id !== id));
}

/**
 * React hook: registers a handler for one action kind and flushes any
 * matching queued actions once (on mount) and again every time the browser
 * fires 'online'. Call once per kind, near where that action is created.
 */
export function useOfflineQueueFlush(kind: string, handler: (payload: unknown) => Promise<void>) {
  useEffect(() => {
    const flush = async () => {
      for (const item of listQueuedActions(kind)) {
        try {
          await handler(item.payload);
          removeFromQueue(item.id);
        } catch {
          break; // still offline or backend down — stop and retry next time
        }
      }
    };
    void flush();
    window.addEventListener("online", flush);
    return () => window.removeEventListener("online", flush);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kind]);
}
