import type { HTMLAttributes, ReactNode } from "react";
import type { DashboardTone } from "./tone";

interface BadgeProps extends Omit<HTMLAttributes<HTMLSpanElement>, "children"> {
  children: ReactNode;
  tone?: DashboardTone;
  dot?: boolean;
}

export function Badge({ children, tone = "neutral", dot = false, className, ...props }: BadgeProps) {
  const classes = ["tw-badge", `tw-badge--${tone}`, className].filter(Boolean).join(" ");

  return (
    <span className={classes} {...props}>
      {dot && <span className="tw-badge__dot" aria-hidden="true" />}
      {children}
    </span>
  );
}
