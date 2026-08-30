import React from "react";

interface StatCardProps {
  label: string;
  value: string | number;
  sub?: string;
  accent?: "gold" | "emerald" | "danger";
  className?: string;
}

export function StatCard({ label, value, sub, accent = "gold", className }: StatCardProps) {
  const classes = ["tw-card", "tw-stat", `tw-stat--${accent}`, className]
    .filter(Boolean)
    .join(" ");

  return (
    <article className={classes}>
      <p className="tw-stat__label">{label}</p>
      <p className="tw-stat__value">{value}</p>
      {sub && <p className="tw-stat__sub">{sub}</p>}
    </article>
  );
}
