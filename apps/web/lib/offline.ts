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

/**
 * A cache round-trip goes through JSON.stringify/parse, which invokes a
 * protobuf-es message's toJSON() and strips its prototype — a cached
 * Timestamp field survives as a plain RFC3339 string, not a class instance
 * with .toDate(). Call sites that render a Timestamp field sourced from
 * cachedFetch must use this instead of a bare `?.toDate()`.
 */
export function toDateSafe(value: unknown): Date | undefined {
  if (!value) return undefined;
  if (value instanceof Date) return value;
  if (typeof value === "string") return new Date(value);
  if (typeof value === "object" && "toDate" in value && typeof (value as { toDate: unknown }).toDate === "function") {
    return (value as { toDate: () => Date }).toDate();
  }
  if (typeof value === "object" && "seconds" in value) {
    const v = value as { seconds: string | number; nanos?: number };
    return new Date(Number(v.seconds) * 1000 + (v.nanos ?? 0) / 1e6);
  }
  return undefined;
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

// `id` doubles as a stable idempotency key: it's generated once at enqueue and
// never changes across replays, so a handler that forwards it to the backend
// lets the server dedupe a replay whose original response was lost after the
// write already committed (the classic at-least-once hazard).
type QueuedAction = { id: string; kind: string; payload: unknown; queuedAt: string; attempts: number };

// After this many failed replays an item is dropped (dead-lettered) instead of
// blocking the queue forever — a permanently-rejected action (e.g. a 4xx) must
// never wedge every later action behind it, which for the SOS queue would mean
// a real emergency alert stuck behind a poison message.
const MAX_QUEUE_ATTEMPTS = 8;
// Soft cap so a long offline stretch can't grow the queue without bound.
const MAX_QUEUE_SIZE = 200;

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

/** Queues a write action for replay when back online. Returns the queued item's id (also its idempotency key). */
export function enqueueAction(kind: string, payload: unknown): string {
  const id = `${kind}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const queue = readQueue();
  queue.push({ id, kind, payload, queuedAt: new Date().toISOString(), attempts: 0 });
  // Drop the oldest overflow if we've somehow blown past the cap.
  writeQueue(queue.slice(-MAX_QUEUE_SIZE));
  return id;
}

function bumpAttempts(id: string): number {
  const queue = readQueue();
  let attempts = 0;
  for (const item of queue) {
    if (item.id === id) {
      item.attempts = (item.attempts ?? 0) + 1;
      attempts = item.attempts;
    }
  }
  writeQueue(queue);
  return attempts;
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
 *
 * The handler receives the queued payload and the item's stable
 * `idempotencyKey` — forward that key to the backend so a replay after a lost
 * response can't create a duplicate. A handler that ignores the second arg
 * still works (the failure just won't be deduped server-side).
 */
export function useOfflineQueueFlush(
  kind: string,
  handler: (payload: unknown, idempotencyKey: string) => Promise<void>,
) {
  useEffect(() => {
    const flush = async () => {
      for (const item of listQueuedActions(kind)) {
        try {
          await handler(item.payload, item.id);
          removeFromQueue(item.id);
        } catch {
          // Give up on a poison item after MAX_QUEUE_ATTEMPTS so it can't
          // wedge every later action forever; otherwise stop and retry the
          // whole batch next time (most likely still offline / backend down).
          if (bumpAttempts(item.id) >= MAX_QUEUE_ATTEMPTS) {
            removeFromQueue(item.id);
            continue;
          }
          break;
        }
      }
    };
    void flush();
    window.addEventListener("online", flush);
    return () => window.removeEventListener("online", flush);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kind]);
}
