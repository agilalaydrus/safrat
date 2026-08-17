"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { getMyAccessCached } from "./access-cache";

// A pilgrim's app_access_code used to live in the URL (/pilgrim/[code]) —
// deliberately removed. Now that pilgrims sign in the same way as
// admin/leader (see the email-matching flow in lib/auth.ts), there's no
// reason to expose that UUID in the address bar at all; it's resolved
// once from the session (via IdentityService.GetMyAccess, already cached)
// and shared with every /pilgrim/* page through this context instead.
const PilgrimCodeContext = createContext<string | undefined>(undefined);

export function PilgrimCodeProvider({ children }: { children: React.ReactNode }) {
  const [code, setCode] = useState<string>();

  useEffect(() => {
    let cancelled = false;
    getMyAccessCached().then((access) => {
      if (!cancelled && access.linkedPilgrim) setCode(access.linkedPilgrim.appAccessCode);
    });
    return () => { cancelled = true; };
  }, []);

  return <PilgrimCodeContext.Provider value={code}>{children}</PilgrimCodeContext.Provider>;
}

/** Empty string while still resolving — callers already guard on `!code` the same way they guarded on a missing URL param before. */
export function usePilgrimCode(): string {
  return useContext(PilgrimCodeContext) ?? "";
}
