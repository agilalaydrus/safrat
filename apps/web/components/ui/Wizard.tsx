"use client";

import type { ReactNode } from "react";
import { IconAlertTriangle, IconCheck } from "@tabler/icons-react";
import { Badge } from "./Badge";
import { ProgressBar } from "./ProgressBar";

export interface WizardStep {
  id: string;
  title: string;
  description?: string;
  complete?: boolean;
}

export interface WizardReadinessCheck {
  id: string;
  label: string;
  passed: boolean;
}

export interface WizardValidationIssue {
  id: string;
  message: string;
  stepId?: string;
}

interface WizardProps {
  title: string;
  steps: readonly WizardStep[];
  currentStepId: string;
  readinessScore: number;
  readinessChecks: readonly WizardReadinessCheck[];
  validationIssues: readonly WizardValidationIssue[];
  children: ReactNode;
  onStepChange?: (stepId: string) => void;
  footer?: ReactNode;
  className?: string;
}

export function Wizard({
  title,
  steps,
  currentStepId,
  readinessScore,
  readinessChecks,
  validationIssues,
  children,
  onStepChange,
  footer,
  className,
}: WizardProps) {
  const boundedScore = Math.min(Math.max(readinessScore, 0), 100);
  const scoreTone = boundedScore >= 80 ? "success" : boundedScore >= 50 ? "warning" : "danger";
  const activeIndex = Math.max(steps.findIndex((step) => step.id === currentStepId), 0);
  const activeStep = steps[activeIndex];
  const classes = ["tw-card", "tw-card--large", "tw-wizard", className].filter(Boolean).join(" ");

  return (
    <section className={classes} aria-label={title}>
      <header className="tw-wizard__header">
        <div>
          <p className="section-eyebrow">Alur Terpandu</p>
          <h1 className="tw-wizard__title">{title}</h1>
        </div>
        <Badge tone={scoreTone}>{boundedScore}% siap</Badge>
      </header>

      <div className="tw-wizard__layout">
        <nav className="tw-wizard__steps" aria-label="Langkah formulir">
          <ol>
            {steps.map((step, index) => {
              const isActive = step.id === currentStepId;
              const isComplete = step.complete ?? index < activeIndex;
              return (
                <li key={step.id}>
                  <button
                    type="button"
                    className="tw-wizard__step"
                    data-active={isActive || undefined}
                    data-complete={isComplete || undefined}
                    aria-current={isActive ? "step" : undefined}
                    onClick={() => onStepChange?.(step.id)}
                    disabled={!onStepChange}
                  >
                    <span className="tw-wizard__step-number" aria-hidden="true">
                      {isComplete ? <IconCheck size={14} /> : index + 1}
                    </span>
                    <span className="tw-wizard__step-copy">
                      <strong>{index + 1} · {step.title}</strong>
                      {step.description && <small>{step.description}</small>}
                    </span>
                  </button>
                </li>
              );
            })}
          </ol>
        </nav>

        <main className="tw-wizard__content">
          {activeStep && <h2 className="tw-wizard__active-title">{activeIndex + 1} · {activeStep.title}</h2>}
          <div className="tw-wizard__body">{children}</div>
        </main>

        <aside className="tw-wizard__readiness" aria-label="Skor kesiapan">
          <div className="tw-wizard__readiness-head">
            <h2>Skor Kesiapan</h2>
            <p>Periksa sebelum melanjutkan ke tindakan akhir.</p>
          </div>
          <ProgressBar
            label="Kelengkapan lintas langkah"
            value={boundedScore}
            valueLabel={`${boundedScore}%`}
            tone={scoreTone}
          />
          <ul className="tw-wizard__checklist">
            {readinessChecks.map((check) => (
              <li key={check.id} data-passed={check.passed || undefined}>
                <span aria-hidden="true">{check.passed ? <IconCheck size={13} /> : <IconAlertTriangle size={13} />}</span>
                {check.label}
              </li>
            ))}
          </ul>

          {validationIssues.length > 0 ? (
            <div className="tw-wizard__issues" role="alert">
              <h3>Perlu diperbaiki lintas langkah</h3>
              <ul>
                {validationIssues.map((issue) => (
                  <li key={issue.id}>
                    {issue.stepId && onStepChange ? (
                      <button type="button" onClick={() => onStepChange(issue.stepId!)}>{issue.message}</button>
                    ) : issue.message}
                  </li>
                ))}
              </ul>
            </div>
          ) : (
            <p className="tw-wizard__valid" role="status"><IconCheck size={14} aria-hidden="true" /> Tidak ada konflik antar langkah.</p>
          )}
        </aside>
      </div>

      {footer && <footer className="tw-wizard__footer">{footer}</footer>}
    </section>
  );
}
