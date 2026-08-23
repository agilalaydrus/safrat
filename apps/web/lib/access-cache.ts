"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import { MyAccess } from "@hajj-saas/proto-gen/hajj/v1/identity_pb";
import { identityClient } from "./rpc";
import { invalidateTokenCache } from "./transport";

/**
 * Client-side twin of the backend's per-user TTL cache in
 * internal/repository/identity.go — the backend cache stops a page-guard
 * hitting the DB with 3 queries on every navigation; this one stops it
 * making a network round trip at all when the same tab visits /dashboard
 * then /leader (each has its own layout, so each mounts its own guard).
 * Real, not stale-forever: short TTL, and cleared on sign-out/role change
 * so it never outlives what it's a cache of.
 */
const ACCESS_CACHE_TTL_MS = 15_000;
const OFFLINE_ACCESS_TTL_MS = 72 * 60 * 60_000;
const OFFLINE_ACCESS_KEY = "safrat:access-snapshot";
let cached: { value: MyAccess; expiresAt: number } | undefined;
let pending: Promise<MyAccess> | undefined;

function readOfflineAccess(): MyAccess | undefined {
  if (typeof window === "undefined") return undefined;
  try {
    const raw = window.localStorage.getItem(OFFLINE_ACCESS_KEY);
    if (!raw) return undefined;
    const stored = JSON.parse(raw) as { value: MyAccess; expiresAt: number };
    if (stored.expiresAt <= Date.now()) {
      window.localStorage.removeItem(OFFLINE_ACCESS_KEY);
      return undefined;
    }
    return stored.value;
  } catch {
    return undefined;
  }
}

function writeOfflineAccess(value: MyAccess) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(
      OFFLINE_ACCESS_KEY,
      JSON.stringify({ value, expiresAt: Date.now() + OFFLINE_ACCESS_TTL_MS }),
    );
  } catch {
    // Best-effort only; a full or unavailable localStorage must not block login.
  }
}

function clearOfflineAccess() {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(OFFLINE_ACCESS_KEY);
  } catch {
    // Nothing else to invalidate when storage is unavailable.
  }
}

export async function getMyAccessCached(): Promise<MyAccess> {
  const now = Date.now();
  if (cached && cached.expiresAt > now) return cached.value;
  if (!pending) {
    pending = identityClient
      .getMyAccess({})
      .then((value) => {
        cached = { value, expiresAt: Date.now() + ACCESS_CACHE_TTL_MS };
        writeOfflineAccess(value);
        return value;
      })
      .catch((error: unknown) => {
        const code = ConnectError.from(error).code;
        if (code === Code.Unauthenticated || code === Code.PermissionDenied) {
          clearOfflineAccess();
          throw error;
        }

        const offlineAccess = readOfflineAccess();
        if (offlineAccess) {
          cached = { value: offlineAccess, expiresAt: Date.now() + ACCESS_CACHE_TTL_MS };
          return offlineAccess;
        }
        throw error;
      })
      .finally(() => { pending = undefined; });
  }
  return pending;
}

/** Call on sign-out and right after any action that changes roles (e.g. linking a Google account). */
export function invalidateMyAccessCache() {
  cached = undefined;
  clearOfflineAccess();
  // The RPC transport caches the session token separately (lib/transport.ts)
  // — without dropping it here too, the next RPC (e.g. the GetMyAccess call
  // right after sign-in) can still carry the previous session's token for
  // up to its TTL and get rejected as unauthenticated.
  invalidateTokenCache();
}
