import React from "react";

interface PageHeroProps {
  eyebrow: string;
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
}

export function PageHero({ eyebrow, title, subtitle, actions }: PageHeroProps) {
  return (
    <header className="tw-page-hero">
      <div className="tw-page-hero__row">
        <div className="tw-page-hero__copy">
          <p className="section-eyebrow">{eyebrow}</p>
          <h1 className="tw-page-hero__title">{title}</h1>
          {subtitle && <p className="tw-page-hero__subtitle">{subtitle}</p>}
        </div>
        {actions && <div className="tw-page-hero__actions">{actions}</div>}
      </div>
      <div className="gold-divider tw-page-hero__divider" />
    </header>
  );
}
