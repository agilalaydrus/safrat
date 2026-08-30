export const DASHBOARD_TONES = [
  "success",
  "info",
  "brand",
  "warning",
  "danger",
  "neutral",
] as const;

export type DashboardTone = (typeof DASHBOARD_TONES)[number];
