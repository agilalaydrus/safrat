import type { ReactNode } from "react";

interface PageHeaderProps {
  title: string;
  subtitle: ReactNode;
  eyebrow?: string;
  controls?: ReactNode;
  primaryAction?: ReactNode;
  className?: string;
}

export function PageHeader({ title, subtitle, eyebrow, controls, primaryAction, className }: PageHeaderProps) {
  const classes = ["tw-page-header", "tw-enter", className].filter(Boolean).join(" ");

  return (
    <header className={classes}>
      <div className="tw-page-header__copy">
        {eyebrow && <p className="section-eyebrow">{eyebrow}</p>}
        <h1 className="tw-page-header__title">{title}</h1>
        <div className="tw-page-header__subtitle">{subtitle}</div>
      </div>
      {(controls || primaryAction) && (
        <div className="tw-page-header__actions">
          {controls && <div className="tw-page-header__controls">{controls}</div>}
          {primaryAction && <div className="tw-page-header__primary-action">{primaryAction}</div>}
        </div>
      )}
    </header>
  );
}
