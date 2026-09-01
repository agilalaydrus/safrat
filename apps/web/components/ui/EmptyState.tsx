import type { ReactNode } from "react";
import Link from "next/link";
import { IconArrowRight, IconInbox } from "@tabler/icons-react";

interface EmptyStateProps {
  title: string;
  cause: string;
  nextStep: string;
  actionHref?: string;
  actionLabel?: string;
  onAction?: () => void;
  icon?: ReactNode;
  className?: string;
}

export function EmptyState({
  title,
  cause,
  nextStep,
  actionHref,
  actionLabel,
  onAction,
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
      {actionLabel && actionHref && <Link className="tw-btn tw-btn--outline tw-btn--sm" href={actionHref}>
        {actionLabel}<IconArrowRight size={14} aria-hidden="true" />
      </Link>}
      {actionLabel && onAction && !actionHref && <button className="tw-btn tw-btn--outline tw-btn--sm" type="button" onClick={onAction}>
        {actionLabel}<IconArrowRight size={14} aria-hidden="true" />
      </button>}
    </section>
  );
}
