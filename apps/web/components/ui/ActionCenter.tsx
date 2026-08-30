import Link from "next/link";
import { IconAlertTriangle, IconArrowRight, IconCircleCheck, IconClockHour4 } from "@tabler/icons-react";
import { Badge } from "./Badge";
import type { DashboardTone } from "./tone";

export interface ActionCenterItem {
  id: string;
  title: string;
  description: string;
  financialImpact: string;
  actionHref: string;
  actionLabel: string;
  tone?: DashboardTone;
}

interface ActionCenterProps {
  items?: readonly ActionCenterItem[];
  title?: string;
  subtitle?: string;
  cleanTitle: string;
  cleanDescription: string;
  error?: string;
  className?: string;
}

export function ActionCenter({
  items,
  title = "Pusat Tindakan",
  subtitle = "Prioritas yang paling perlu ditindaklanjuti hari ini",
  cleanTitle,
  cleanDescription,
  error,
  className,
}: ActionCenterProps) {
  const classes = ["tw-card", "tw-card--large", "tw-action-center", "tw-enter", className]
    .filter(Boolean)
    .join(" ");

  return (
    <section className={classes} aria-label={title}>
      <header className="tw-action-center__header">
        <div>
          <h2 className="tw-action-center__title">{title}</h2>
          <p className="tw-action-center__subtitle">{subtitle}</p>
        </div>
        {error ? (
          <Badge tone="danger">Data gagal</Badge>
        ) : items === undefined ? (
          <Badge tone="neutral">Memuat</Badge>
        ) : (
          <Badge tone={items.length ? "warning" : "success"}>{items.length ? `${items.length} tindakan` : "Bersih"}</Badge>
        )}
      </header>

      {error ? (
        <div className="tw-action-center__clean" role="alert">
          <div className="tw-action-center__clean-icon tw-action-center__clean-icon--danger" aria-hidden="true"><IconAlertTriangle size={22} /></div>
          <div>
            <h3>Prioritas belum dapat dihitung</h3>
            <p>{error}</p>
          </div>
        </div>
      ) : items === undefined ? (
        <div className="tw-action-center__clean" role="status">
          <div className="tw-action-center__clean-icon tw-action-center__clean-icon--loading" aria-hidden="true"><IconClockHour4 size={22} /></div>
          <div>
            <h3>Menghitung prioritas</h3>
            <p>Menunggu data musim yang sedang dipilih.</p>
          </div>
        </div>
      ) : items.length ? (
        <ul className="tw-action-center__list tw-stagger">
          {items.map((item) => (
            <li key={item.id} className="tw-action-center__item tw-enter">
              <div className="tw-action-center__copy">
                <div className="tw-action-center__item-heading">
                  <h3>{item.title}</h3>
                  <Badge tone={item.tone ?? "warning"}>{item.financialImpact}</Badge>
                </div>
                <p>{item.description}</p>
              </div>
              <Link href={item.actionHref} className="tw-action-center__link">
                {item.actionLabel}
                <IconArrowRight size={14} aria-hidden="true" />
              </Link>
            </li>
          ))}
        </ul>
      ) : (
        <div className="tw-action-center__clean" role="status">
          <div className="tw-action-center__clean-icon" aria-hidden="true"><IconCircleCheck size={22} /></div>
          <div>
            <h3>{cleanTitle}</h3>
            <p>{cleanDescription}</p>
          </div>
        </div>
      )}
    </section>
  );
}
