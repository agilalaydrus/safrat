"use client";

import { RequireAccess } from "@/components/auth/RequireAccess";

export default function LeaderLayout({ children }: { children: React.ReactNode }) {
  return <RequireAccess role="leader">{children}</RequireAccess>;
}
