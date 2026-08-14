Task: Redesign all form sheet dialogs — fix sticky header, white inputs, softer shape, custom controls

The current form dialogs look stiff, the "Close" button scrolls out of view as the user fills in the form,
and input fields use a murky cream background. Apply the fixes below to ALL form dialog components.

Files to update:
  apps/web/components/pilgrims/PilgrimFormDialog.tsx
  apps/web/components/accommodation/HotelFormDialog.tsx
  apps/web/components/transport/MovementFormDialog.tsx
  apps/web/components/transport/VehicleFormDialog.tsx
  apps/web/app/globals.css

Read each file before editing.

---

CHANGE 1 — Sheet container shape

Old:
  const sheet: React.CSSProperties = {
    width: "min(560px,100%)", overflowY: "auto", background: "var(--color-cream-100)",
    padding: 24, boxShadow: "-4px 0 16px rgba(26,20,16,.1)", animation: "sheet-in .2s ease-out"
  };

New (apply to ALL dialogs):
  const sheet: React.CSSProperties = {
    width: "min(560px,100%)",
    height: "100vh",
    display: "flex",
    flexDirection: "column",
    background: "#ffffff",
    boxShadow: "-6px 0 32px rgba(26,20,16,.12)",
    borderRadius: "16px 0 0 16px",
    animation: "sheet-in .22s cubic-bezier(0,0,.2,1)",
    overflow: "hidden",
  };

---

CHANGE 2 — Sticky header with X close button

Replace every dialog's header section. Currently header is just a div at the top of the sheet.
Make it a sticky header that stays visible when the form scrolls.

Old header style:
  const header: React.CSSProperties = { display: "flex", alignItems: "start", justifyContent: "space-between", gap: 16 };

New styles:
  const stickyHeader: React.CSSProperties = {
    position: "sticky",
    top: 0,
    zIndex: 10,
    background: "#ffffff",
    borderBottom: "1px solid var(--color-cream-300)",
    padding: "20px 24px 16px",
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 16,
    flexShrink: 0,
  };

  const closeBtn: React.CSSProperties = {
    width: 40,
    height: 40,
    borderRadius: "50%",
    border: "1px solid var(--color-cream-400)",
    background: "transparent",
    color: "var(--color-warm-400)",
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    cursor: "pointer",
    flexShrink: 0,
    transition: "background .15s, color .15s",
  };

Replace the old "Close" text button with an icon button:
  import { IconX } from "@tabler/icons-react";
  ...
  <button type="button" onClick={() => requestClose()} style={closeBtn} aria-label="Close">
    <IconX size={18} />
  </button>

Add to globals.css for hover:
  .btn-close-sheet:hover { background: var(--color-cream-200) !important; color: var(--color-warm-900) !important; }

Add className="btn-close-sheet" to the close button.

---

CHANGE 3 — Form body becomes scrollable flex child

Wrap the <form> (or content below sticky header) in a scrollable div:

  const formBody: React.CSSProperties = {
    flex: 1,
    overflowY: "auto",
    padding: "24px",
    display: "grid",
    gap: 0,
  };

The outer <aside style={sheet}> now has:
  1. <div style={stickyHeader}>...</div>   ← sticky, never scrolls
  2. <div style={formBody}>               ← scrollable
       <form>...</form>
     </div>

---

CHANGE 4 — Sticky submit button at bottom

Wrap the submit button in a sticky footer:

  const stickyFooter: React.CSSProperties = {
    position: "sticky",
    bottom: 0,
    background: "#ffffff",
    borderTop: "1px solid var(--color-cream-300)",
    padding: "16px 24px",
    flexShrink: 0,
  };

Move the submit <button> OUT of the <form> grid and into a <div style={stickyFooter}> below the scrollable body.
Use form="pilgrim-form" attribute on the button to link it to the form, and add id="pilgrim-form" to the <form> tag.
(Each dialog has its own form id: "pilgrim-form", "hotel-form", "movement-form", "vehicle-form")

The submit button itself stays full-width gold:
  const primary: React.CSSProperties = {
    minHeight: 48, border: 0, borderRadius: 10, background: "var(--color-gold-500)",
    color: "var(--color-warm-900)", fontWeight: 700, padding: "0 20px", cursor: "pointer",
    fontFamily: "'Plus Jakarta Sans', sans-serif", fontSize: 14, width: "100%",
  };

---

CHANGE 5 — Input fields: white background, more rounded, focus ring

Old:
  const input: React.CSSProperties = {
    minHeight: 48, width: "100%", border: "1px solid var(--color-cream-500)",
    borderRadius: 8, padding: "10px 12px", background: "var(--color-cream-200)",
    font: "inherit", color: "var(--color-warm-900)"
  };

New:
  const input: React.CSSProperties = {
    minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)",
    borderRadius: 10, padding: "0 14px", background: "#ffffff",
    font: "inherit", color: "var(--color-warm-900)",
    outline: "none", transition: "border-color .15s, box-shadow .15s",
  };

Add focus styles via globals.css (inline styles can't do :focus):
  .safrat-input:focus {
    border-color: var(--color-emerald-800) !important;
    box-shadow: 0 0 0 3px rgba(13,61,39,.10);
  }
  .safrat-input[aria-invalid="true"] {
    border-color: var(--color-danger-600) !important;
    box-shadow: 0 0 0 3px rgba(220,38,38,.08);
  }
  .safrat-input::placeholder { color: var(--color-warm-400); }

Add className="safrat-input" to every <input>, <select>, <textarea> in all dialogs.

---

CHANGE 6 — Field labels: darker and more legible

Old Field component label color was warm-500 (too muted).

New Field label style:
  <span style={{ fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)", display: "block", marginBottom: 6 }}>
    {label}{required && <span style={{ color: "var(--color-danger-600)", marginLeft: 2, fontWeight: 400 }}>*</span>}
  </span>

---

CHANGE 7 — Form section headers: small caps, NOT big serif

Old Section component in PilgrimFormDialog:
  function Section({ title, children }) {
    return <section style={{ display: "grid", gap: 12 }}>
      <h3 style={{ margin: 0, fontSize: 20 }}>{title}</h3>   ← TOO LARGE
      {children}
    </section>;
  }

New:
  function Section({ title, children }: { title: string; children: React.ReactNode }) {
    return (
      <section style={{ display: "grid", gap: 16 }}>
        <p style={{
          margin: 0, fontSize: 11, fontWeight: 700, letterSpacing: ".1em",
          textTransform: "uppercase", color: "var(--color-warm-400)",
          paddingBottom: 8, borderBottom: "1px solid var(--color-cream-300)",
          fontFamily: "'Plus Jakarta Sans', sans-serif",
        }}>
          {title}
        </p>
        {children}
      </section>
    );
  }

And the sectionDivider between sections:
  const sectionDivider: React.CSSProperties = { paddingTop: 28 };
  (remove the borderTop — the Section header itself now has the border-bottom)

---

CHANGE 8 — Custom radio buttons for Gender field (PilgrimFormDialog)

Replace the plain browser radio buttons with styled ones.
Add these styles to globals.css:

  .safrat-radio-group { display: flex; gap: 12px; }
  .safrat-radio-label {
    display: flex; align-items: center; gap: 10px;
    min-height: 44px; padding: 0 14px;
    border: 1.5px solid var(--color-cream-400); border-radius: 10px;
    cursor: pointer; flex: 1; font-size: 14px;
    color: var(--color-warm-700); transition: border-color .15s, background .15s;
  }
  .safrat-radio-label:hover { border-color: var(--color-emerald-200); background: var(--color-emerald-50); }
  .safrat-radio-label.selected {
    border-color: var(--color-emerald-800);
    background: var(--color-emerald-50);
    color: var(--color-emerald-900); font-weight: 600;
  }
  .safrat-radio-label input[type="radio"] { display: none; }
  .safrat-radio-dot {
    width: 18px; height: 18px; border-radius: 50%;
    border: 1.5px solid var(--color-cream-400);
    display: flex; align-items: center; justify-content: center; flex-shrink: 0;
    transition: border-color .15s;
  }
  .safrat-radio-label.selected .safrat-radio-dot {
    border-color: var(--color-emerald-800);
  }
  .safrat-radio-dot-inner {
    width: 10px; height: 10px; border-radius: 50%;
    background: var(--color-emerald-900);
    opacity: 0; transition: opacity .15s;
  }
  .safrat-radio-label.selected .safrat-radio-dot-inner { opacity: 1; }

Replace the Gender fieldset in PilgrimFormDialog with:
  <div className="safrat-radio-group">
    {(["MALE", "FEMALE"] as const).map((value) => (
      <label
        key={value}
        className={`safrat-radio-label${form.gender === value ? " selected" : ""}`}
        onClick={() => update("gender", value)}
      >
        <input type="radio" name="gender" checked={form.gender === value} onChange={() => update("gender", value)} />
        <span className="safrat-radio-dot"><span className="safrat-radio-dot-inner" /></span>
        {value === "MALE" ? "Male" : "Female"}
      </label>
    ))}
  </div>

---

CHANGE 9 — Overlay backdrop: slightly blurred feel

Old overlay:
  const overlay: React.CSSProperties = { position: "fixed", inset: 0, background: "rgba(26,20,16,.52)", zIndex: 20, display: "flex", justifyContent: "flex-end" };

New:
  const overlay: React.CSSProperties = {
    position: "fixed", inset: 0, zIndex: 20,
    display: "flex", justifyContent: "flex-end",
    background: "rgba(26,20,16,.48)",
    backdropFilter: "blur(2px)",
    WebkitBackdropFilter: "blur(2px)",
  };

---

CHANGE 10 — globals.css: update sheet-in animation

Old:
  @keyframes sheet-in {
    from { transform: translateX(100%); opacity: 0; }
    to   { transform: translateX(0);   opacity: 1; }
  }

New (remove opacity flash, use only translate):
  @keyframes sheet-in {
    from { transform: translateX(105%); }
    to   { transform: translateX(0); }
  }

Also add the new CSS classes from changes above:
  .btn-close-sheet:hover { background: var(--color-cream-200) !important; color: var(--color-warm-900) !important; }
  .safrat-input:focus { border-color: var(--color-emerald-800) !important; box-shadow: 0 0 0 3px rgba(13,61,39,.10); }
  .safrat-input[aria-invalid="true"] { border-color: var(--color-danger-600) !important; box-shadow: 0 0 0 3px rgba(220,38,38,.08); }
  .safrat-input::placeholder { color: var(--color-warm-400); }
  .safrat-radio-group { display: flex; gap: 12px; }
  .safrat-radio-label { display: flex; align-items: center; gap: 10px; min-height: 44px; padding: 0 14px; border: 1.5px solid var(--color-cream-400); border-radius: 10px; cursor: pointer; flex: 1; font-size: 14px; color: var(--color-warm-700); transition: border-color .15s, background .15s; }
  .safrat-radio-label:hover { border-color: var(--color-emerald-200); background: var(--color-emerald-50); }
  .safrat-radio-label.selected { border-color: var(--color-emerald-800); background: var(--color-emerald-50); color: var(--color-emerald-900); font-weight: 600; }
  .safrat-radio-label input[type="radio"] { display: none; }
  .safrat-radio-dot { width: 18px; height: 18px; border-radius: 50%; border: 1.5px solid var(--color-cream-400); display: flex; align-items: center; justify-content: center; flex-shrink: 0; transition: border-color .15s; }
  .safrat-radio-label.selected .safrat-radio-dot { border-color: var(--color-emerald-800); }
  .safrat-radio-dot-inner { width: 10px; height: 10px; border-radius: 50%; background: var(--color-emerald-900); opacity: 0; transition: opacity .15s; }
  .safrat-radio-label.selected .safrat-radio-dot-inner { opacity: 1; }

---

APPLY ORDER

1. Update globals.css first (all CSS classes + keyframes)
2. Update PilgrimFormDialog.tsx (most complex — has Section component + radio)
3. Update HotelFormDialog.tsx
4. Update MovementFormDialog.tsx
5. Update VehicleFormDialog.tsx

For dialogs 3 & 4, apply: sheet shape, sticky header with X icon, scrollable body, sticky footer, white inputs with .safrat-input class, darker labels, section headers as small caps.

VERIFICATION

After changes:
  pnpm --filter web build

Visually check:
  - Scroll inside the form → header with X button stays pinned at top
  - Submit button stays pinned at bottom
  - Click input → green border ring appears (not blue browser default)
  - Invalid input → red border ring appears
  - Gender radio buttons look like pill toggle options, not browser defaults
  - Sheet left edge has 16px border-radius
  - Background behind sheet is blurred
