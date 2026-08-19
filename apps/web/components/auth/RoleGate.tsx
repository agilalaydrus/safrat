"use client";

import { OrgRole, useMyRole } from "@/lib/use-my-role";

export function RoleGate({ require, children, fallback = null }: { require: OrgRole | OrgRole[]; children: React.ReactNode; fallback?: React.ReactNode }) {
  const { role, isPending } = useMyRole();
  if (isPending) return null;
  const allowedRoles = Array.isArray(require) ? require : [require];
  if (!role || !allowedRoles.includes(role)) return <>{fallback}</>;
  return <>{children}</>;
}
