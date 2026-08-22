"use client";

import { useEffect } from "react";
import { pilgrimAppClient } from "@/lib/rpc";
import { requestPushToken } from "@/lib/firebase";

/** Registers this device for push notifications (location updates, ritual
 * completions, kloter milestones) once per app open — silently does
 * nothing if Firebase isn't configured or permission is denied, same as
 * requestPushToken itself. */
export function useRegisterPilgrimPush(appAccessCode: string | undefined) {
  useEffect(() => {
    if (!appAccessCode) return;
    let cancelled = false;
    requestPushToken().then((token) => {
      if (!cancelled && token) pilgrimAppClient.registerMyPushToken({ appAccessCode, fcmToken: token }).catch(() => {});
    });
    return () => { cancelled = true; };
  }, [appAccessCode]);
}
