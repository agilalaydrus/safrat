import type { ReactNode } from "react";
import Link from "next/link";
import { IconArrowRight, IconInbox } from "@tabler/icons-react";

interface EmptyStateProps {
  title: string;
  cause: string;
  nextStep: string;
  actionHref: string;
  actionLabel: string;
  icon?: ReactNode;
  className?: string;
}

export function EmptyState({
  title,
  cause,
  nextStep,
  actionHref,
  actionLabel,
  icon,
  className,
}: EmptyStateProps) {
  const classes = ["tw-empty-state", className].filter(Boolean).join(" ");

  return (
    <section className={classes} aria-label={title}>
      <div className="tw-empty-state__icon" aria-hidden="true">{icon ?? <IconInbox size={22} />}</div>
      <div className="tw-empty-state__copy">
        <h3 className="tw-empty-state__title">{title}</h3>
        <p className="tw-empty-state__cause">{cause}</p>
        <p className="tw-empty-state__next"><strong>Langkah berikutnya:</strong> {nextStep}</p>
      </div>
      <Link className="tw-btn tw-btn--outline tw-btn--sm" href={actionHref}>
        {actionLabel}
        <IconArrowRight size={14} aria-hidden="true" />
      </Link>
    </section>
  );
}
