import type { ReactNode } from "react";
import { IconInfoCircle } from "@tabler/icons-react";

interface MethodologyNoteProps {
  summary: ReactNode;
  points?: readonly ReactNode[];
  title?: string;
  className?: string;
}

export function MethodologyNote({
  summary,
  points = [],
  title = "Catatan Metodologi",
  className,
}: MethodologyNoteProps) {
  const classes = ["tw-methodology", className].filter(Boolean).join(" ");

  return (
    <aside className={classes} aria-label={title}>
      <div className="tw-methodology__icon" aria-hidden="true"><IconInfoCircle size={19} /></div>
      <div className="tw-methodology__copy">
        <h2 className="tw-methodology__title">{title}</h2>
        <div className="tw-methodology__summary">{summary}</div>
        {points.length > 0 && (
          <ul className="tw-methodology__points">
            {points.map((point, index) => <li key={index}>{point}</li>)}
          </ul>
        )}
      </div>
    </aside>
  );
}
