"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { LeaderGroup } from "@hajj-saas/proto-gen/hajj/v1/groupleader_pb";
import { groupLeaderClient } from "./rpc";

// A leader's group id used to live in the URL (/leader/[groupId]) —
// removed for the same reason as the pilgrim access code. Most leaders
// lead exactly one group, so there's usually nothing to pick; a leader
// with more than one gets a switcher (rendered by whichever page needs
// it) instead of a UUID in the address bar. The selection persists in
// sessionStorage so it survives navigation between /leader/* pages
// without needing to be encoded in the URL.
const STORAGE_KEY = "safrat:leader:selected-group";

type LeaderContextValue = {
  groups: LeaderGroup[];
  loaded: boolean;
  selectedGroupId: string;
  setSelectedGroupId: (id: string) => void;
};

const LeaderContext = createContext<LeaderContextValue>({ groups: [], loaded: false, selectedGroupId: "", setSelectedGroupId: () => {} });

export function LeaderGroupProvider({ children }: { children: React.ReactNode }) {
  const [groups, setGroups] = useState<LeaderGroup[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [selectedGroupId, setSelectedGroupIdState] = useState("");

  useEffect(() => {
    groupLeaderClient.listMyGroups({}).then((response) => {
      setGroups(response.groups);
      setLoaded(true);
      const stored = typeof window !== "undefined" ? window.sessionStorage.getItem(STORAGE_KEY) : null;
      const initial = response.groups.find((g) => g.id === stored)?.id ?? response.groups[0]?.id ?? "";
      setSelectedGroupIdState(initial);
    }).catch(() => setLoaded(true));
  }, []);

  function setSelectedGroupId(id: string) {
    setSelectedGroupIdState(id);
    window.sessionStorage.setItem(STORAGE_KEY, id);
  }

  return <LeaderContext.Provider value={{ groups, loaded, selectedGroupId, setSelectedGroupId }}>{children}</LeaderContext.Provider>;
}

export function useLeaderGroup() {
  return useContext(LeaderContext);
}
