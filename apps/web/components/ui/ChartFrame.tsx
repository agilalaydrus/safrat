import type { ReactNode } from "react";

interface ChartFrameProps {
  title: string;
  axisDescription: string;
  children: ReactNode;
  legend?: ReactNode;
  action?: ReactNode;
  className?: string;
}

export function ChartFrame({ title, axisDescription, children, legend, action, className }: ChartFrameProps) {
  const classes = ["tw-card", "tw-card--large", "tw-chart-frame", "tw-enter", className]
    .filter(Boolean)
    .join(" ");

  return (
    <figure className={classes} aria-label={title}>
      <figcaption className="tw-chart-frame__header">
        <div className="tw-chart-frame__copy">
          <h2 className="tw-chart-frame__title">{title}</h2>
          <p className="tw-chart-frame__axis">{axisDescription}</p>
        </div>
        {action && <div className="tw-chart-frame__action">{action}</div>}
      </figcaption>
      <div className="tw-chart-frame__plot">{children}</div>
      {legend && <div className="tw-chart-frame__legend" aria-label="Legenda grafik">{legend}</div>}
    </figure>
  );
}
