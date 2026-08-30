"use client";

import { useEffect, useId, useRef, type ReactNode } from "react";
import { IconX } from "@tabler/icons-react";

interface DetailDrawerProps {
  open: boolean;
  onClose: () => void;
  title: string;
  subtitle?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  closeLabel?: string;
  className?: string;
}

const FOCUSABLE_SELECTOR = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

export function DetailDrawer({
  open,
  onClose,
  title,
  subtitle,
  children,
  footer,
  closeLabel = "Tutup panel detail",
  className,
}: DetailDrawerProps) {
  const titleId = useId();
  const drawerRef = useRef<HTMLElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;

    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeRef.current?.focus();

    function handleKeyDown(event: globalThis.KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
        return;
      }
      if (event.key !== "Tab" || !drawerRef.current) return;

      const focusable = Array.from(drawerRef.current.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
      if (focusable.length === 0) {
        event.preventDefault();
        return;
      }

      const first = focusable[0]!;
      const last = focusable[focusable.length - 1]!;
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = previousOverflow;
      previousFocus?.focus();
    };
  }, [onClose, open]);

  if (!open) return null;

  const drawerClasses = ["tw-detail-drawer", className].filter(Boolean).join(" ");

  return (
    <div className="tw-detail-drawer-layer">
      <button className="tw-detail-drawer__backdrop" type="button" onClick={onClose} aria-label={closeLabel} />
      <aside ref={drawerRef} className={drawerClasses} role="dialog" aria-modal="true" aria-labelledby={titleId}>
        <header className="tw-detail-drawer__header">
          <div>
            <h2 id={titleId} className="tw-detail-drawer__title">{title}</h2>
            {subtitle && <div className="tw-detail-drawer__subtitle">{subtitle}</div>}
          </div>
          <button ref={closeRef} className="tw-detail-drawer__close" type="button" onClick={onClose} aria-label={closeLabel}>
            <IconX size={19} aria-hidden="true" />
          </button>
        </header>
        <div className="tw-detail-drawer__body">{children}</div>
        {footer && <footer className="tw-detail-drawer__footer">{footer}</footer>}
      </aside>
    </div>
  );
}
