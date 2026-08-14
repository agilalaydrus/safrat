Task: Fix all Critical + High UI issues in Safrat

Read these files first before making any changes:
- apps/web/components/accommodation/AccommodationDashboard.tsx
- apps/web/components/transport/TransportDashboard.tsx
- apps/web/components/accommodation/HotelFormDialog.tsx
- apps/web/components/pilgrims/PilgrimFormDialog.tsx
- apps/web/app/dashboard/(shell)/layout.tsx
- apps/web/app/globals.css

---

GLOBAL CSS VAR REPLACEMENT MAP — apply to ALL files. These vars no longer exist:

--border-default         → --color-cream-400
--bg-input               → --color-cream-200
--bg-surface-alt         → white
--bg-surface             → --color-cream-100
--text-secondary         → --color-warm-500
--text-primary           → --color-warm-900
--text-gold              → --color-gold-800
--text-emerald           → --color-emerald-900
--text-muted             → --color-warm-400
--action-primary-bg      → --color-gold-500
--action-primary-text    → --color-warm-900
--action-secondary-bg    → --color-emerald-900
--action-secondary-text  → --color-cream-100
--shadow-xs              → remove the property entirely
--color-info-50          → --color-cream-200

---

FIX 1 — apps/web/components/accommodation/AccommodationDashboard.tsx

Apply the full var map to every inline style. Also:

- manage link style: background: "var(--color-emerald-50)", color: "var(--color-emerald-900)"
- empty state: borderColor: "var(--color-cream-400)", background: "var(--color-cream-100)"
- Add const [search, setSearch] = useState("") to component state
- Change the grouped useMemo to filter hotels by search before grouping:
  hotels.filter(h => h.name.toLowerCase().includes(search.toLowerCase()) || h.city.toLowerCase().includes(search.toLowerCase()))
- Insert this search input right after <div className="gold-divider" /> when hotels are loaded:

{!loading && hotels.length > 0 && (
  <input
    value={search}
    onChange={e => setSearch(e.target.value)}
    placeholder="Search hotels..."
    style={{
      minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8,
      padding: "0 14px", background: "var(--color-cream-200)", font: "inherit",
      color: "var(--color-warm-900)", width: "100%", maxWidth: 360, margin: "16px 0 0"
    }}
  />
)}

---

FIX 2 — apps/web/components/transport/TransportDashboard.tsx

This file is broken AND minified to a single line. Rewrite it as a clean, properly indented component. Preserve all existing logic (season selector, listMovements, group-by-date, MovementFormDialog). Apply the full var map. Replace the badge() function with:

function badge(status: string): React.CSSProperties {
  const map: Record<string, [string, string]> = {
    arrived:   ["var(--color-emerald-50)",  "var(--color-emerald-900)"],
    departed:  ["var(--color-cream-200)",   "var(--color-warm-500)"],
    cancelled: ["#fdf0f0",                  "var(--color-danger-600)"],
  };
  const [bg, color] = map[status] ?? ["var(--color-cream-300)", "var(--color-warm-500)"];
  return {
    justifySelf: "start", padding: "5px 10px", borderRadius: 99,
    background: bg, color, textTransform: "capitalize", fontSize: 12
  };
}

---

FIX 3 — apps/web/components/accommodation/HotelFormDialog.tsx

Replace the single error string state with a field-level errors map:

const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

Add this validate() function:

function validate(): Record<string, string> {
  const errs: Record<string, string> = {};
  if (!form.name.trim()) errs.name = "Hotel name is required.";
  if (!form.city) errs.city = "City is required.";
  if (form.checkInDate && form.checkOutDate && form.checkOutDate < form.checkInDate)
    errs.checkOutDate = "Check-out must be on or after check-in date.";
  return errs;
}

In submit(): call validate() first — if errors exist, call setFieldErrors(errs) and return. On API catch, set fieldErrors._form. Clear fieldErrors at the start of each submit.

Update the Field component to accept and render an error prop:

function Field({ label, required, error, children }: {
  label: string; required?: boolean; error?: string; children: React.ReactNode
}) {
  return (
    <label style={{ display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 }}>
      {label}{required && <span style={{ color: "var(--color-danger-600)" }}> *</span>}
      {children}
      {error && <span style={{ fontSize: 11, color: "var(--color-danger-600)" }}>{error}</span>}
    </label>
  );
}

Pass errors to each field:
- <Field label="Hotel name" required error={fieldErrors.name}>
- <Field label="City" required error={fieldErrors.city}>
- <Field label="Check-out date" error={fieldErrors.checkOutDate}>

Replace the old {error && <p role="alert"...>} with:

{fieldErrors._form && (
  <p role="alert" style={{
    margin: 0, fontSize: 13, color: "var(--color-danger-600)",
    background: "#fdf0f0", padding: "10px 12px", borderRadius: 8
  }}>
    {fieldErrors._form}
  </p>
)}

---

FIX 4 — apps/web/components/pilgrims/PilgrimFormDialog.tsx

The Group ID field asks for a UUID but the Groups module doesn't exist yet. Change it to a plain text group code field:

Replace the groupId Field with:

<Field
  fieldKey="groupId"
  label="Group code"
  hint="Short label to identify the group (e.g. GRP-A). Leave blank to assign later."
>
  <input
    value={form.groupId}
    onChange={e => update("groupId", e.target.value)}
    placeholder="e.g. GRP-A"
    style={input}
  />
</Field>

In validate(), delete this entire block:

if (form.groupId && !UUID_PATTERN.test(form.groupId.trim())) {
  errs.groupId = "Group ID must be a valid UUID. Leave it blank to assign later.";
}

---

FIX 5 — apps/web/app/dashboard/(shell)/layout.tsx

Add a title tooltip to each "Coming soon" nav item:

<span
  title="Coming soon — this module is in development"
  style={disabled}
>
  <Icon size={18} />{label}<b style={soon}>Soon</b>
</span>

Add to apps/web/app/globals.css:

.btn-signout:hover { color: rgba(255,255,255,0.9) !important; }

Add className="btn-signout" to the sign-out <button> element.

---

VERIFICATION

After all changes, run:

pnpm --filter web build

Then confirm zero broken vars remain:

grep -r "\-\-border-default\|\-\-bg-input\|\-\-bg-surface\|\-\-text-secondary\|\-\-text-primary\|\-\-text-gold\|\-\-text-emerald\|\-\-action-primary-bg\|\-\-action-secondary-bg\|\-\-shadow-xs\|\-\-color-info-50" apps/web/components/ apps/web/app/

This command must return zero matches.
