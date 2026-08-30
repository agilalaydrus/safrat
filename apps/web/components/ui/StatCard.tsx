import { Badge } from "./Badge";
import type { DashboardTone } from "./tone";

interface StatDelta {
  value: string;
  label?: string;
  direction?: "up" | "down" | "neutral";
}

interface StatCardProps {
  label: string;
  value: string | number;
  unit: string;
  delta?: StatDelta;
  sparkline?: readonly number[];
  sparklineLabel?: string;
  tone?: DashboardTone;
  className?: string;
}

function sparklinePoints(values: readonly number[]): string {
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;

  return values
    .map((value, index) => {
      const x = values.length === 1 ? 50 : (index / (values.length - 1)) * 100;
      const y = 28 - ((value - min) / range) * 24;
      return `${x},${y}`;
    })
    .join(" ");
}

export function StatCard({
  label,
  value,
  unit,
  delta,
  sparkline,
  sparklineLabel,
  tone = "brand",
  className,
}: StatCardProps) {
  const classes = ["tw-card", "tw-stat", "tw-enter", `tw-stat--${tone}`, className]
    .filter(Boolean)
    .join(" ");
  const canDrawSparkline = Boolean(sparkline && sparkline.length > 0);

  return (
    <article className={classes}>
      <p className="tw-stat__label">{label}</p>
      <div className="tw-stat__main">
        <p className="tw-stat__value">
          <span>{value}</span>
          <span className="tw-stat__unit">{unit}</span>
        </p>
        {canDrawSparkline && (
          <svg
            className="tw-stat__sparkline"
            viewBox="0 0 100 32"
            preserveAspectRatio="none"
            role={sparklineLabel ? "img" : undefined}
            aria-label={sparklineLabel}
            aria-hidden={sparklineLabel ? undefined : true}
          >
            <polyline className="tw-chart-motion" points={sparklinePoints(sparkline!)} vectorEffect="non-scaling-stroke" />
          </svg>
        )}
      </div>
      {delta && (
        <div className="tw-stat__delta">
          <Badge tone={delta.direction === "down" ? "danger" : delta.direction === "up" ? "success" : "neutral"}>
            {delta.direction === "up" ? "↑ " : delta.direction === "down" ? "↓ " : ""}{delta.value}
          </Badge>
          {delta.label && <span>{delta.label}</span>}
        </div>
      )}
    </article>
  );
}
