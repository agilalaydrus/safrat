"use client";

import type { KeyboardEvent, MouseEvent, ReactNode } from "react";
import { IconDownload, IconSearch } from "@tabler/icons-react";

export interface DataTableColumn<Row> {
  id: string;
  header: ReactNode;
  cell: (row: Row) => ReactNode;
  align?: "left" | "center" | "right";
  className?: string;
}

interface DataTableProps<Row> {
  ariaLabel: string;
  columns: readonly DataTableColumn<Row>[];
  rows: readonly Row[];
  getRowId: (row: Row) => string;
  searchValue: string;
  onSearchChange: (value: string) => void;
  searchPlaceholder: string;
  filters?: ReactNode;
  exportHref?: string;
  exportLabel?: string;
  onRowClick?: (row: Row) => void;
  getRowLabel?: (row: Row) => string;
  emptyState: ReactNode;
  loading?: boolean;
  className?: string;
}

const INTERACTIVE_SELECTOR = "a,button,input,select,textarea,[role='button'],[role='link']";

export function DataTable<Row>({
  ariaLabel,
  columns,
  rows,
  getRowId,
  searchValue,
  onSearchChange,
  searchPlaceholder,
  filters,
  exportHref,
  exportLabel = "Ekspor",
  onRowClick,
  getRowLabel,
  emptyState,
  loading = false,
  className,
}: DataTableProps<Row>) {
  const classes = ["tw-card", "tw-card--large", "tw-data-table", className].filter(Boolean).join(" ");

  function handleRowClick(event: MouseEvent<HTMLTableRowElement>, row: Row) {
    if (!onRowClick || (event.target as HTMLElement).closest(INTERACTIVE_SELECTOR)) return;
    onRowClick(row);
  }

  function handleRowKeyDown(event: KeyboardEvent<HTMLTableRowElement>, row: Row) {
    if (!onRowClick || (event.target as HTMLElement).closest(INTERACTIVE_SELECTOR)) return;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onRowClick(row);
    }
  }

  return (
    <section className={classes} aria-label={ariaLabel}>
      <div className="tw-data-table__toolbar">
        <label className="tw-data-table__search">
          <span className="sr-only">Cari dalam {ariaLabel}</span>
          <IconSearch size={17} aria-hidden="true" />
          <input
            type="search"
            value={searchValue}
            onChange={(event) => onSearchChange(event.target.value)}
            placeholder={searchPlaceholder}
          />
        </label>
        {filters && <div className="tw-data-table__filters">{filters}</div>}
        {exportHref && (
          <a className="tw-btn tw-btn--outline tw-btn--sm" href={exportHref}>
            <IconDownload size={15} aria-hidden="true" />
            {exportLabel}
          </a>
        )}
      </div>

      <div className="tw-data-table__summary" aria-live="polite">
        {loading ? "Memuat data…" : `${rows.length.toLocaleString("id-ID")} hasil`}
        {onRowClick && !loading && rows.length > 0 && <span>Klik baris untuk membuka detail</span>}
      </div>

      {loading ? (
        <div className="tw-data-table__loading" role="status">Memuat data…</div>
      ) : rows.length === 0 ? (
        <div className="tw-data-table__empty">{emptyState}</div>
      ) : (
        <div className="tw-data-table__scroller">
          <table>
            <thead>
              <tr>
                {columns.map((column) => (
                  <th key={column.id} scope="col" className={column.className} data-align={column.align ?? "left"}>{column.header}</th>
                ))}
              </tr>
            </thead>
            <tbody className="tw-stagger">
              {rows.map((row) => (
                <tr
                  key={getRowId(row)}
                  className={onRowClick ? "tw-data-table__clickable-row tw-enter" : "tw-enter"}
                  tabIndex={onRowClick ? 0 : undefined}
                  aria-label={getRowLabel?.(row)}
                  onClick={(event) => handleRowClick(event, row)}
                  onKeyDown={(event) => handleRowKeyDown(event, row)}
                >
                  {columns.map((column) => (
                    <td key={column.id} className={column.className} data-align={column.align ?? "left"}>{column.cell(row)}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
