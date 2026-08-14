# Fix: Remove Top Header on Desktop + Redesign Mobile Header

Read these two files first:
  apps/web/app/dashboard/(shell)/layout.tsx
  apps/web/app/globals.css

---

## PROBLEM

The `mobileHeader` element has no media query to hide it on desktop. This causes both
the left sidebar AND the top bar to be visible at the same time on desktop screens,
creating a redundant, cluttered layout. The sidebar already contains the logo, operator
name, and season — the top bar adds nothing on desktop.

On mobile the header is also too rigid (flat rectangle, harsh contrast).

---

## CHANGE 1 — globals.css: hide mobile header on desktop

Add this rule after the existing `[data-desktop-sidebar]` rule:

```css
@media (min-width: 768px) {
  [data-mobile-header] { display: none !important; }
}
```

The existing file already has:
  @media (min-width: 768px) { [data-desktop-sidebar] { display: block !important; } }

Add the new rule on the very next line, right after that block.

---

## CHANGE 2 — layout.tsx: add data-mobile-header + redesign

In the layout return JSX, find the `<header style={mobileHeader}>` element and:

1. Add `data-mobile-header=""` attribute to it
2. Replace the `mobileHeader` style constant with the new design below
3. Redesign the inner content (new structure below)

### New mobileHeader style

Replace:
```ts
const mobileHeader:React.CSSProperties={display:"flex",justifyContent:"space-between",alignItems:"center",padding:"16px 20px",background:"var(--color-emerald-900)",position:"sticky",top:0,zIndex:40};
```

With:
```ts
const mobileHeader: React.CSSProperties = {
  display: "flex",
  justifyContent: "space-between",
  alignItems: "center",
  padding: "12px 16px",
  background: "rgba(13,61,39,0.92)",
  backdropFilter: "blur(12px)",
  WebkitBackdropFilter: "blur(12px)",
  position: "sticky",
  top: 0,
  zIndex: 40,
  borderBottom: "1px solid rgba(201,168,76,0.20)",
};
```

### New mobile header JSX

Replace the old `<header ...>` contents:
```tsx
<button aria-label="Open menu" onClick={() => setOpen(true)} style={menu}><IconMenu2 size={24} /></button>
<span style={{ ...logo, fontSize: 20 }}>Safrat</span>
<i style={{ width: 48 }} />
```

With this new structure:
```tsx
<button
  aria-label="Open menu"
  onClick={() => setOpen(true)}
  style={menu}
>
  <IconMenu2 size={22} />
</button>

<div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 1 }}>
  <span style={{ ...logo, fontSize: 18, lineHeight: 1 }}>Safrat</span>
  {season && (
    <span style={{
      fontSize: 10,
      color: "rgba(255,255,255,0.55)",
      letterSpacing: "0.06em",
      fontFamily: "'Plus Jakarta Sans', sans-serif",
      fontWeight: 500,
    }}>
      {season}
    </span>
  )}
</div>

<div style={{ width: 40 }} />
```

### Updated menu button style

Replace:
```ts
const menu:React.CSSProperties={width:48,minHeight:48,border:0,background:"transparent",color:"var(--color-cream-100)"};
```

With:
```ts
const menu: React.CSSProperties = {
  width: 40,
  height: 40,
  border: "1px solid rgba(201,168,76,0.25)",
  borderRadius: 10,
  background: "rgba(255,255,255,0.06)",
  color: "var(--color-cream-100)",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  flexShrink: 0,
};
```

---

## RESULT

- Desktop (≥768px): top bar is completely hidden. Only the left sidebar is visible. Content starts immediately below the page fold — no wasted space.
- Mobile (<768px): soft frosted-glass emerald bar, rounded hamburger button with gold border, centered logo + season name stacked underneath, no rigid hard edges.

---

## VERIFICATION

After changes:
  pnpm --filter web build

Then check:
  - On desktop (≥768px): NO top bar visible. Content fills full width below sidebar.
  - On mobile / narrow viewport: top bar shows with frosted glass effect.
  - Sidebar still shows on desktop with logo, org name, season, nav.
  - Mobile drawer still works (hamburger opens sidebar overlay).
