import type { CSSProperties } from "react";
import type { DashboardTone } from "./tone";

interface ProgressBarProps {
  label: string;
  value: number;
  max?: number;
  unit?: string;
  valueLabel?: string;
  tone?: DashboardTone;
  className?: string;
}

export function ProgressBar({
  label,
  value,
  max = 100,
  unit,
  valueLabel,
  tone = "brand",
  className,
}: ProgressBarProps) {
  const safeMax = max > 0 ? max : 1;
  const boundedValue = Math.min(Math.max(value, 0), safeMax);
  const percentage = (boundedValue / safeMax) * 100;
  const displayValue = valueLabel ?? `${value.toLocaleString("id-ID")}${unit ? ` ${unit}` : ""}`;
  const classes = ["tw-progress", `tw-progress--${tone}`, className].filter(Boolean).join(" ");
  const fillStyle = { "--tw-progress-value": `${percentage}%` } as CSSProperties;

  return (
    <div className={classes}>
      <div className="tw-progress__meta">
        <span className="tw-progress__label">{label}</span>
        <span className="tw-progress__value">{displayValue}</span>
      </div>
      <div
        className="tw-progress__track"
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={safeMax}
        aria-valuenow={boundedValue}
        aria-valuetext={displayValue}
      >
        <span className="tw-progress__fill tw-progress-motion" style={fillStyle} />
      </div>
    </div>
  );
}
