# Safrat — UI Design Specification

> **Purpose:** Single source of truth for all UI/UX generation.
> Every Codex session generating UI must read this file alongside `CODEX_SPEC.md`.
> Do not deviate from tokens, components, or screen layouts defined here.

---

## 1. Brand Identity

| | |
|---|---|
| **Product name** | Safrat (سَفْرَة) |
| **Meaning** | "A journey" — one intentional trip, not generic travel |
| **Tagline** | *Where every journey begins* |
| **Voice** | Warm, trustworthy, precise — like a seasoned guide, not a government office |
| **Feel** | Luxury hospitality meets modern SaaS — think Jumeirah hotel app, not generic enterprise dashboard |

### Logo direction (for designer handoff)
- Wordmark: **Safrat** in serif, paired with a small geometric crescent-moon or miqat arch motif
- Icon mark: Stylized arch/gate (miqat) with a subtle gold accent
- Never use clipart Ka'bah or generic mosque silhouette — too cliché
- Works in: full color (emerald + gold), single color (emerald), reversed (cream on emerald)

---

## 2. Design Principles

**1. Clarity before cleverness**
Every element serves a function. No decoration for decoration's sake. If it doesn't help the user act, remove it.

**2. Dignified, not stiff**
Luxury means restraint, not coldness. Rounded corners, generous whitespace, warm tones — approachable but refined.

**3. One primary action per screen**
Every screen has one clear next step. Supporting actions are secondary or hidden until needed.

**4. Age-inclusive**
Design for 55-year-old first-time smartphone users without condescending to 25-year-old coordinators. Large tap targets, clear labels, no icon-only buttons on mobile.

**5. RTL-first for Arabic**
Arabic is the primary language of the market. Every layout must work in RTL. Use logical CSS properties (`margin-inline-start`, `padding-inline-end`) — never `margin-left/right` hardcoded.

---

## 3. Design Tokens

### 3.1 Color Palette

```css
/* Emerald — primary brand color */
--color-emerald-950: #061f14;
--color-emerald-900: #0d3d27;  /* Primary — deep, luxurious */
--color-emerald-800: #1a5c3a;  /* Hover state */
--color-emerald-700: #246b47;
--color-emerald-200: #a8c9b5;  /* Light border */
--color-emerald-100: #d4ede1;  /* Light background tint */
--color-emerald-50:  #e8f4ee;  /* Subtle tag/badge bg */

/* Gold — luxury accent */
--color-gold-900: #5a3e10;
--color-gold-800: #8a6820;     /* Dark gold text */
--color-gold-600: #b8972a;     /* Gold stroke/icon */
--color-gold-500: #c9a84c;     /* Primary gold — buttons, accents */
--color-gold-400: #d4b86a;     /* Hover gold */
--color-gold-100: #fef3c7;     /* Light gold bg */
--color-gold-50:  #fef8e6;     /* Subtle gold tint */

/* Cream — warm background system */
--color-cream-100: #fdf9f0;    /* Page background */
--color-cream-200: #f5f0e8;    /* Card / surface */
--color-cream-300: #ede5d4;    /* Divider / border subtle */
--color-cream-400: #e0d4b0;    /* Border default */
--color-cream-500: #d4c49a;    /* Border strong */

/* Warm neutrals */
--color-warm-900: #1a1410;     /* Primary text — warm black */
--color-warm-700: #3d3027;     /* Secondary heading */
--color-warm-500: #6b5a47;     /* Body secondary */
--color-warm-400: #9a8860;     /* Muted / placeholder */
--color-warm-200: #c4b99a;     /* Disabled */
--color-warm-100: #e8e0d0;     /* Divider */

/* Semantic */
--color-danger-900: #7f1d1d;
--color-danger-700: #991b1b;
--color-danger-600: #dc2626;   /* SOS button, error state */
--color-danger-100: #fdf0f0;
--color-danger-50:  #fef2f2;

--color-success-700: #065f46;
--color-success-600: #059669;
--color-success-50:  #ecfdf5;

--color-info-700: #1e40af;
--color-info-50:  #dbeafe;
```

### 3.2 Semantic Tokens (use these in components — not raw palette)

```css
/* Backgrounds */
--bg-page:        var(--color-cream-100);   /* Page canvas */
--bg-surface:     var(--color-cream-200);   /* Card, panel */
--bg-surface-alt: #ffffff;                  /* Modal, popover */
--bg-input:       var(--color-cream-200);   /* Input field */

/* Text */
--text-primary:   var(--color-warm-900);
--text-secondary: var(--color-warm-500);
--text-muted:     var(--color-warm-400);
--text-disabled:  var(--color-warm-200);
--text-inverse:   #f5f0e8;                  /* Text on dark surfaces */
--text-gold:      var(--color-gold-800);
--text-emerald:   var(--color-emerald-900);

/* Borders */
--border-subtle:  var(--color-cream-300);
--border-default: var(--color-cream-400);
--border-strong:  var(--color-cream-500);
--border-gold:    var(--color-gold-500);
--border-emerald: var(--color-emerald-200);
--border-focus:   var(--color-emerald-800);

/* Brand actions */
--action-primary-bg:    var(--color-gold-500);
--action-primary-hover: var(--color-gold-400);
--action-primary-text:  var(--color-warm-900);
--action-secondary-bg:  var(--color-emerald-900);
--action-secondary-hover: var(--color-emerald-800);
--action-secondary-text: var(--text-inverse);
--action-danger-bg:     var(--color-danger-600);
--action-danger-text:   #ffffff;
```

### 3.3 Typography

```
Display font:  Playfair Display (serif) — headings, screen titles, brand name
Body font:     Plus Jakarta Sans (sans-serif) — all body text, labels, inputs
Arabic font:   IBM Plex Arabic — Arabic script (RTL)
Mono font:     JetBrains Mono — codes, passport numbers, reference IDs
```

**Scale:**
```
display-2xl:  48px / 500 / line-height 1.1   — Hero, landing only
display-xl:   36px / 500 / line-height 1.15  — Page hero titles
display-lg:   28px / 500 / line-height 1.2   — Section title (serif)
heading-xl:   22px / 500 / line-height 1.3   — Card heading
heading-lg:   18px / 500 / line-height 1.35  — Sub-section
heading-md:   16px / 500 / line-height 1.4   — Inline heading
body-lg:      17px / 400 / line-height 1.65  — Primary body (mobile)
body-md:      15px / 400 / line-height 1.6   — Standard body
body-sm:      13px / 400 / line-height 1.55  — Supporting copy
label-lg:     14px / 500 / line-height 1     — Form labels, nav
label-sm:     11px / 500 / line-height 1     — Uppercase caps labels
caption:      12px / 400 / line-height 1.5   — Metadata, timestamps
mono:         13px / 400 / line-height 1     — IDs, passport numbers
```

**Rule:** All headings (display-*, heading-*) use Playfair Display (serif).
Body, labels, inputs, buttons use Plus Jakarta Sans.
Uppercase caps labels: `letter-spacing: 0.08em`.

### 3.4 Spacing

```
2px   — xs      (icon gap, tight inline)
4px   — sm      (badge padding)
8px   — md      (component internal gap)
12px  — lg      (card internal gap)
16px  — xl      (section gap small)
20px  — 2xl     (card padding)
24px  — 3xl     (section gap)
32px  — 4xl     (section spacing)
48px  — 5xl     (page section gap)
```

### 3.5 Border Radius

```
4px   — xs   — tags, badges, small chips
8px   — sm   — buttons, inputs, small cards
12px  — md   — standard cards, panels
16px  — lg   — mobile cards, bottom sheets
20px  — xl   — mobile screen containers, modals
50%   — full — avatars, icon circles
```

**Rule:** Never use sharp 0px corners anywhere — minimum 4px.
Mobile components use larger radius (lg/xl) than web (md).

### 3.6 Shadows

```css
/* Flat luxury — no dramatic drop shadows */
--shadow-none:   none;
--shadow-xs:     0 1px 2px rgba(26,20,16,0.06);
--shadow-sm:     0 2px 8px rgba(26,20,16,0.08);
--shadow-md:     0 4px 16px rgba(26,20,16,0.10);
--shadow-gold:   0 0 0 2px rgba(201,168,76,0.25);  /* Gold focus ring */
--shadow-focus:  0 0 0 3px rgba(26,92,58,0.20);    /* Emerald focus ring */
```

### 3.7 Tailwind Config (apps/web + NativeWind)

```typescript
// tailwind.config.ts
export default {
  theme: {
    extend: {
      colors: {
        emerald: {
          50: '#e8f4ee', 100: '#d4ede1', 200: '#a8c9b5',
          700: '#246b47', 800: '#1a5c3a', 900: '#0d3d27', 950: '#061f14',
        },
        gold: {
          50: '#fef8e6', 100: '#fef3c7', 400: '#d4b86a',
          500: '#c9a84c', 600: '#b8972a', 800: '#8a6820', 900: '#5a3e10',
        },
        cream: {
          100: '#fdf9f0', 200: '#f5f0e8', 300: '#ede5d4',
          400: '#e0d4b0', 500: '#d4c49a',
        },
        warm: {
          100: '#e8e0d0', 200: '#c4b99a', 400: '#9a8860',
          500: '#6b5a47', 700: '#3d3027', 900: '#1a1410',
        },
      },
      fontFamily: {
        serif: ['Playfair Display', 'Georgia', 'serif'],
        sans:  ['Plus Jakarta Sans', 'system-ui', 'sans-serif'],
        arabic: ['IBM Plex Arabic', 'sans-serif'],
        mono:  ['JetBrains Mono', 'monospace'],
      },
      borderRadius: {
        xs: '4px', sm: '8px', md: '12px', lg: '16px', xl: '20px',
      },
    },
  },
}
```

---

## 4. Component Library

> All web components built on **shadcn/ui** + Tailwind. All mobile components on **NativeWind**.
> Override shadcn defaults with Safrat tokens — do not use shadcn defaults as-is.

### 4.1 Buttons

**Rules:**
- Minimum height: 48px (mobile), 44px (web)
- Minimum width: 120px
- Always label + icon — never icon-only on mobile
- Max 2 primary buttons per screen — use secondary/ghost for the rest

```typescript
// Variants
type ButtonVariant = 'gold' | 'emerald' | 'outline-gold' | 'ghost' | 'danger'
type ButtonSize = 'sm' | 'md' | 'lg'

// Gold — primary CTA (save, confirm, submit)
// bg: gold-500, text: warm-900, hover: gold-400
// border-radius: sm (8px)

// Emerald — secondary primary (navigate, view list)
// bg: emerald-900, text: cream-200, hover: emerald-800

// Outline-gold — tertiary (export, download, secondary action)
// bg: transparent, border: gold-500 1px, text: gold-800

// Ghost — cancel, dismiss
// bg: transparent, border: cream-500 0.5px, text: warm-500

// Danger — delete, irreversible actions only
// bg: danger-600, text: white — require confirmation modal before execution
```

**Size scale:**
```
sm: height 40px, padding 0 16px, font 13px, radius 8px
md: height 48px, padding 0 24px, font 14px, radius 8px  ← default
lg: height 56px, padding 0 32px, font 16px, radius 10px ← mobile primary CTA
```

### 4.2 Input Fields

```
Height:       48px (mobile), 44px (web)
Border:       1px solid cream-500
Border-focus: 1.5px solid emerald-800 + shadow-focus
Background:   cream-100 (page) / cream-200 (card)
Radius:       8px
Font:         body-md (15px), Plus Jakarta Sans
Placeholder:  warm-400
```

**States:**
- Default: `border-cream-500`
- Focus: `border-emerald-800` + green focus ring
- Error: `border-danger-600` + red helper text below
- Disabled: `bg-cream-300`, `text-disabled`, `cursor-not-allowed`

**Always pair with:**
- Label above (never placeholder-only)
- Helper text below when needed (not tooltip)
- Error message below on invalid (not inline alert)

### 4.3 Cards

```typescript
// Standard card
className="bg-cream-200 border border-cream-500/50 rounded-md p-5"
// hover: bg-cream-200 border-cream-500 shadow-sm (if clickable)

// Dark card (agent dashboard, highlight section)
className="bg-emerald-900 border border-gold-500/25 rounded-md p-5"

// Stat/metric card
className="bg-cream-100 border border-cream-400/50 rounded-sm p-4"
// metric value: 24px, font-weight 500
// metric label: 11px, uppercase, warm-400, letter-spacing 0.08em

// Alert card (SOS, warning)
className="bg-danger-50 border border-danger-600/30 rounded-md p-4"
```

**Gold accent divider** — use inside cards to separate sections:
```css
/* Between card header and content */
height: 1px;
background: linear-gradient(90deg, transparent, gold-500 at 40%, transparent);
margin: 16px 0;
```

### 4.4 Status Tags / Badges

```
Height:     26px
Padding:    4px 12px
Radius:     full (20px)
Font:       12px, weight 500, letter-spacing 0.02em
Always:     include a leading icon (14px)
```

```typescript
type TagVariant = 'emerald' | 'gold' | 'danger' | 'info' | 'neutral'

// emerald: bg-emerald-50, text-emerald-900, border-emerald-200
// gold:    bg-gold-50,    text-gold-800,    border-gold-100
// danger:  bg-danger-50,  text-danger-700,  border-danger-100
// info:    bg-info-50,    text-info-700,    border-info-100
// neutral: bg-cream-200,  text-warm-500,    border-cream-400
```

### 4.5 Navigation

**Web — Sidebar (Operator Dashboard)**
```
Width:          240px (expanded), 64px (collapsed)
Background:     emerald-900
Active item:    bg-emerald-800, left border 2px gold-500, text cream-200
Inactive item:  text warm-200/60, hover bg-emerald-800/50
Icon:           20px, always with label (never icon-only)
Logo area:      32px Safrat serif in cream-200, gold ornament below
Bottom:         User avatar + name + logout
```

**Mobile — Bottom Tab Bar (Group Leader + Pilgrim App)**
```
Height:         64px + safe area inset
Background:     cream-200
Border-top:     0.5px cream-500
Active icon:    emerald-900, label emerald-900, dot indicator gold-500
Inactive:       warm-400
Font:           10px, uppercase, letter-spacing 0.06em
Max tabs:       4 (Pilgrim App), 5 (Group Leader App)
```

### 4.6 Data Table (Operator Dashboard)

```
Header:        bg-cream-200, text warm-400 11px uppercase, border-bottom cream-500
Row:           bg-transparent, border-bottom cream-300/50, 52px height
Row hover:     bg-cream-200/50
Selected row:  bg-emerald-50, border-left 2px gold-500
Text:          body-md warm-900
Muted column:  body-sm warm-400 (passport number, timestamps)
Pagination:    bottom right, ghost buttons, current page gold-500
Sort indicator: gold-500 arrow icon
```

### 4.7 Modal / Bottom Sheet

**Web Modal:**
```
Overlay:      rgba(26,20,16,0.5) backdrop
Container:    bg white, radius xl (20px), shadow-md, max-width 480px
Header:       serif heading-xl, gold ornament divider below
Footer:       right-aligned buttons, ghost + gold/emerald
```

**Mobile Bottom Sheet:**
```
Handle bar:   4px wide, 32px tall, warm-200, centered, margin 8px
Radius:       xl (20px) top corners only
Background:   cream-100
Max height:   85vh
```

### 4.8 Form Sheet Dialog (Web — Right Slide Panel) ← UPDATED v2.1

The form sheet is a right-side slide-in panel (not a centered modal). These rules apply to ALL form dialogs: PilgrimFormDialog, HotelFormDialog, MovementFormDialog, ProductFormDialog, AgentFormDialog.

**Sheet container:**
```
Width:            min(560px, 100vw)
Background:       #ffffff  ← pure white, NOT cream-100 (too yellow)
Shadow:           -6px 0 32px rgba(26,20,16,0.12)
Left edge radius: 16px 0 0 16px  ← soften the entry edge
Animation:        translateX(100%)→0, 220ms cubic-bezier(0,0,0.2,1)
```

**Sheet header — MUST be position: sticky; top: 0**
```
Background:    #ffffff
z-index:       10
Padding:       20px 24px 16px
Border-bottom: 1px solid var(--color-cream-300)
Layout:        flex, space-between, align-items center

Left side:
  Eyebrow:     10px, uppercase, letter-spacing 0.1em, gold-600, margin 0 0 4px
  Title:       Playfair Display, 22px, weight 500, emerald-900, margin 0

Right side:
  Close button: 40×40px circle icon button
    icon:       X (IconX from tabler, 20px)
    style:      border: 1px solid cream-400, borderRadius: 50%,
                background: transparent, color: warm-400
    hover:      background: cream-200, color: warm-900
    label:      aria-label="Close"
  ← NEVER use text-only "Close" button — use icon X in a circle
```

**Form body (below sticky header):**
```
Padding:       24px
Display:       grid, gap 0 (sections handle their own spacing)
Overflow-y:    auto
```

**Section blocks inside form:**
```
Section header:   NOT Playfair, NOT 20px
  font:           Plus Jakarta Sans, 11px, weight 700, uppercase, letter-spacing 0.1em
  color:          warm-400
  margin:         0 0 16px
  padding-bottom: 8px
  border-bottom:  1px solid cream-300

Section gap:      28px between sections (paddingTop on each section after first)
```

**Input fields — UPDATED:**
```
Height:           48px
Background:       #ffffff  ← white, NOT cream-200
Border:           1.5px solid var(--color-cream-400)
Border-radius:    10px  ← was 8px
Padding:          0 14px
Font:             15px, Plus Jakarta Sans, warm-900
Placeholder:      warm-400

Focus state:
  border-color:   var(--color-emerald-800)
  box-shadow:     0 0 0 3px rgba(13,61,39,0.10)  ← soft emerald ring
  outline:        none

Error state:
  border-color:   var(--color-danger-600)
  box-shadow:     0 0 0 3px rgba(220,38,38,0.08)

Disabled state:
  background:     var(--color-cream-200)
  border-color:   var(--color-cream-300)
  color:          var(--color-warm-400)
  cursor:         not-allowed
```

**Field labels:**
```
Font:    13px, weight 600, Plus Jakarta Sans
Color:   var(--color-warm-700)  ← was warm-500 (too muted)
Margin:  0 0 6px
Required asterisk: color danger-600, margin-left 2px, font-weight 400
```

**Field helper/hint text:**
```
Font:    12px, weight 400
Color:   var(--color-warm-400)
Margin:  4px 0 0
```

**Field error text:**
```
Font:    12px, weight 500
Color:   var(--color-danger-600)
Margin:  4px 0 0
```

**Submit button — MUST be sticky at bottom:**
```css
position: sticky;
bottom: 0;
background: #ffffff;
padding: 16px 24px;
border-top: 1px solid var(--color-cream-300);
/* Button inside is full-width gold */
```

**Textarea:**
```
Same as input, but:
  min-height: 96px
  padding:    12px 14px
  resize:     vertical
```

**Select:**
```
Same as input
appearance: none
background-image: chevron SVG (warm-400)
padding-right: 40px
```

**Radio buttons (gender):**
```
Custom styled, not browser default
Each option: 44px height, flex row, gap 10px, align-items center
Radio circle: 18×18px, border 1.5px cream-400, border-radius 50%
  Checked: border emerald-800, inner dot 10×10 emerald-900
  Hover: border emerald-600
Label: 14px, warm-700
```

**Checkbox:**
```
18×18px, border 1.5px cream-400, border-radius 4px
Checked: background emerald-900, white checkmark
Label: 14px, warm-700, margin-left 10px
```

### 4.8 Empty States

```
Icon:         48px, warm-200
Heading:      heading-md, serif, warm-700
Body:         body-sm, warm-400, max 2 lines
CTA button:   gold, centered
Margin-top:   64px from top of content area
```

Never: "Tidak ada data" alone without explanation and action.
Always: tell the user what to do next.

---

## 5. Screen Inventory

### 5.1 Operator Dashboard (Web — Next.js)

| Screen | Route | Primary Action | Key Data |
|---|---|---|---|
| Sign in | `/sign-in` | Sign in with email | — |
| Onboarding (3 steps) | `/onboarding` | Complete setup | Company info, first season, invite |
| Dashboard home | `/dashboard` | — | Season summary, SOS alerts, quick stats |
| Pilgrim list | `/dashboard/pilgrims` | Add pilgrim / Import CSV | Table: name, passport, group, room, status |
| Pilgrim detail | `/dashboard/pilgrims/[id]` | Edit / Substitute | All allocations, docs, history |
| Pilgrim import | `/dashboard/pilgrims/import` | Upload CSV → confirm | Column mapping, row validation |
| Group list | `/dashboard/groups` | Create group | Groups with leader, pilgrim count |
| Group detail | `/dashboard/groups/[id]` | Add pilgrim to group | Pilgrim list, leader card, movement |
| Hotel list | `/dashboard/accommodation` | Add hotel | Hotels with room summary |
| Hotel detail | `/dashboard/accommodation/[id]` | Add room / Allocate | Room grid with occupancy |
| Transport | `/dashboard/transport` | Create movement | Movements, vehicles, seat map |
| Products | `/dashboard/products` | Add product | Product list, orders, commission |
| Agent list | `/dashboard/agents` | — | Agent table, earnings, tier |
| Agent detail | `/dashboard/agents/[id]` | Trigger payout | Dashboard: earnings, referrals, orders |
| SOS monitor | `/dashboard/sos` | Acknowledge | Real-time alert list, map if available |
| Reports | `/dashboard/reports` | Export | Hotel manifest, bus manifest, pilgrim list |
| Settings — Season | `/dashboard/settings/seasons` | Create season | Season list, active toggle |
| Settings — Team | `/dashboard/settings/team` | Invite user | User list, roles |
| Settings — Profile | `/dashboard/settings/profile` | Save | Company info, branding |

### 5.2 Group Leader App (Mobile — Expo)

| Screen | Route | Primary Action | Offline? |
|---|---|---|---|
| Login | `/(auth)/login` | Enter group code | — |
| My Group | `/(tabs)/index` | — | ✅ |
| Pilgrim detail | `/pilgrim/[id]` | Call / View allocations | ✅ |
| Attendance | `/(tabs)/attendance` | Mark present | ✅ |
| Check-in QR | `/(tabs)/checkin` | Show QR / manual tap | ✅ |
| Movement | `/(tabs)/movement` | — | ✅ |
| Products | `/(tabs)/products` | Order for pilgrim | ❌ |
| SOS received | `/sos/[id]` | Acknowledge | ❌ push-triggered |
| Chat | `/chat` | Send message | ✅ queue |

### 5.3 Pilgrim App (Mobile — Expo)

| Screen | Route | Primary Action | Offline? |
|---|---|---|---|
| Onboard / scan | `/(auth)/scan` | Scan QR / enter code | — |
| Home | `/(tabs)/home` | — | ✅ |
| SOS | `/(tabs)/sos` | Press SOS button | ✅ queue |
| Chat | `/(tabs)/chat` | Send message | ✅ queue |
| Schedule | `/(tabs)/schedule` | — | ✅ |
| Products | `/(tabs)/products` | Request product | ❌ |

### 5.4 Pilgrim PWA (Web — Next.js `/pilgrim/[code]`)

| Screen | Path | Primary Action |
|---|---|---|
| Home | `/pilgrim/[code]` | — |
| SOS | `/pilgrim/[code]/sos` | Press SOS |
| Chat | `/pilgrim/[code]/chat` | Send message |
| Schedule | `/pilgrim/[code]/schedule` | — |

---

## 6. Layout Patterns

### 6.1 Web — Operator Dashboard Layout

```
┌─────────────────────────────────────────────┐
│  Sidebar 240px  │  Main content area         │
│  emerald-900    │  bg-page (cream-100)        │
│                 │  ┌──────────────────────┐   │
│  [Logo]         │  │ Page header           │  │
│  ─────          │  │ serif title + breadcrumb│ │
│  Nav items      │  │ gold ornament divider │  │
│  ...            │  ├──────────────────────┤   │
│                 │  │ Content               │  │
│  ─────          │  │                       │  │
│  [User]         │  └──────────────────────┘   │
└─────────────────────────────────────────────┘

Page header:
  - Serif heading-xl (emerald-900)
  - Breadcrumb in body-sm warm-400
  - Primary action button (gold) top-right
  - Gold ornament divider below header

Content area padding: 32px all sides
Card gap: 16px
```

### 6.2 Mobile — Standard Screen Layout

```
┌──────────────────┐
│ Status bar        │
│ ─────────────── │
│ Screen header     │  48px — title center, back arrow left
│ ─────────────── │  0.5px cream-500 divider
│                   │
│  Content          │  padding 20px horizontal
│  (scrollable)     │
│                   │
│ ─────────────── │
│ Bottom tab bar    │  64px + safe area
└──────────────────┘

Screen header:
  - serif heading-lg (emerald-900), center aligned
  - Back arrow: left (LTR), right (RTL)
  - Optional right action: icon button (gold-500)
```

### 6.3 Mobile — Pilgrim App Specific

```
Max taps to any primary action: 2
Home screen must contain: group info + next movement + SOS access
SOS button: always visible on home screen — never behind a menu
Navigation: bottom tab only — no hamburger, no drawer
Font minimum: 16px body, 14px labels — nothing smaller
```

---

## 7. Persona-Specific UI Rules

### Operations Manager
- Dense data tables with filtering, sorting, export
- Financial summary cards with gold accent numbers
- Batch actions (select multiple pilgrims → assign group)
- Print-ready manifest preview

### Operations Coordinator
- Quick-access search bar always visible at top
- Color-coded room occupancy grid (green = occupied, cream = empty, red = over-capacity)
- Movement status: departed / in-transit / arrived chips
- SOS alert banner always at top if any alert is active (danger-50 bg, danger-600 border)

### Group Leader / Mutawwif
- Offline indicator banner: subtle cream-300 top bar with wifi-off icon when offline
- Pilgrim list: large names, 56px row height, avatar initial circle
- Attendance: full-screen mode — one tap = mark present, green checkmark animation
- SOS received: full-screen takeover — red bg, pilgrim name, GPS coords, acknowledge button

### Pilgrim
- Max 2 taps to any action from home screen
- SOS button: full-width, 72px height, danger-600 bg, uppercase text — unmissable
- No menus, no drawers, no nested navigation
- Font: body-lg (17px) minimum throughout — accessibility for 45-65yo
- Language selector: prominent on first screen, remembered after

### Referral Agent
- Earnings in large gold numbers
- Tier badge: STANDARD (warm), SILVER (cool gray), GOLD (gold) — prominent in profile header
- Referral link: large copy button + share button side by side
- Commission history: timeline style, not table

---

## 8. Multi-Language & RTL

### Supported languages
| Code | Language | Script | Direction |
|---|---|---|---|
| `ar` | Arabic | Arabic | RTL |
| `id` | Indonesian | Latin | LTR |
| `ur` | Urdu | Nastaliq | RTL |
| `bn` | Bengali | Bengali | LTR |
| `tr` | Turkish | Latin | LTR |

### RTL implementation rules

```typescript
// NEVER use margin-left/right or padding-left/right directly
// ALWAYS use logical properties:

// ✅ Correct
margin-inline-start: 16px;
padding-inline-end: 12px;
border-inline-start: 2px solid gold-500;
text-align: start;

// ❌ Wrong
margin-left: 16px;
padding-right: 12px;
border-left: 2px solid gold-500;
text-align: left;

// In Tailwind: use ms-* me-* ps-* pe-* instead of ml-* mr-* pl-* pr-*
```

```typescript
// In Next.js — set dir on html element
// app/layout.tsx
<html lang={locale} dir={locale === 'ar' || locale === 'ur' ? 'rtl' : 'ltr'}>

// In Expo — use I18nManager
import { I18nManager } from 'react-native'
I18nManager.forceRTL(isRTL)
```

### Arabic font rules
```css
/* Use IBM Plex Arabic only for Arabic text blocks */
/* Fallback to system Arabic font if not loaded */
[lang="ar"], [lang="ur"] {
  font-family: 'IBM Plex Arabic', 'Geeza Pro', 'Arabic Typesetting', sans-serif;
  line-height: 1.8;  /* Arabic needs more line-height */
}
```

### Number formatting
- Always use `Intl.NumberFormat` with locale
- In Arabic locale: use Eastern Arabic numerals (`٠١٢٣٤٥٦٧٨٩`) for counts
- Currency: always `SAR` prefix or `ريال` suffix in Arabic

---

## 9. Motion & Animation

**Philosophy:** Purposeful, not decorative. Motion confirms action, guides attention — never loops or entertains.

```typescript
// Duration tokens
const duration = {
  instant:  0,        // Toggle, checkbox
  fast:     100,      // Tooltip appear
  base:     200,      // Button press, state change  ← default
  slow:     350,      // Modal open, bottom sheet slide
  xslow:    500,      // Page transition
}

// Easing
const easing = {
  standard:    'cubic-bezier(0.4, 0, 0.2, 1)',   // Most transitions
  decelerate:  'cubic-bezier(0, 0, 0.2, 1)',      // Entering elements
  accelerate:  'cubic-bezier(0.4, 0, 1, 1)',      // Leaving elements
}
```

**Specific animations:**
```
Button press:        scale(0.97), duration 100ms
Modal open:          opacity 0→1 + translateY 8px→0, duration 200ms, decelerate
Bottom sheet slide:  translateY 100%→0, duration 350ms, decelerate
Page transition:     opacity 0→1, duration 200ms
SOS button press:    scale(0.95) + shake (3 cycles, 150ms total) — confirms trigger
Attendance check:    Green circle expand + checkmark draw, duration 300ms
Offline banner:      slideDown 48px, duration 200ms
```

**Reduced motion:**
```css
@media (prefers-reduced-motion: reduce) {
  * { animation-duration: 0.01ms !important; transition-duration: 0.01ms !important; }
}
```

---

## 10. Accessibility

### Touch targets
```
Minimum tap target:  48×48px (mobile) — non-negotiable
Minimum web click:   36×36px
SOS button:          full-width × 72px — cannot be smaller
Bottom nav items:    64px height (whole zone is tappable)
```

### Color contrast
```
Body text on cream bg: warm-900 on cream-100 → contrast ratio ≥ 7:1 ✅
Muted text:            warm-400 on cream-100 → min 3:1 (decorative only)
Gold text:             gold-800 on cream-50  → min 4.5:1 ✅
White text on emerald: cream-200 on emerald-900 → ≥ 7:1 ✅
Danger text:           danger-700 on danger-50 → ≥ 4.5:1 ✅
```

### Focus states
```css
/* All interactive elements MUST have visible focus */
:focus-visible {
  outline: 2px solid var(--color-emerald-800);
  outline-offset: 2px;
  border-radius: inherit;
}
/* Gold variant focus (on dark/emerald bg) */
.on-dark:focus-visible {
  outline-color: var(--color-gold-500);
}
```

### Screen readers
- Every icon button needs `aria-label`
- Status tags: include `role="status"` for dynamic updates
- SOS confirmation: `role="alertdialog"`, `aria-live="assertive"`
- Loading states: `aria-busy="true"` on container

### Font size
- Web minimum: 13px (caption) — no smaller
- Mobile minimum: 14px (label-sm) — no smaller
- Pilgrim App minimum: 16px body throughout — hard rule

---

## 11. Dark Mode

> Dark mode is **not in scope for v1**. All screens use the cream/light palette.
> Design tokens are structured to support dark mode in v2 — use semantic tokens, not raw hex values.
> Operator Dashboard may ship dark mode in v2 (power users working nights).
> Pilgrim App and Group Leader App: light only (outdoor use in Saudi sun — light mode is more readable).

---

## 12. Icon System

**Library:** Tabler Icons (outline only — never filled)
**Size scale:**
```
16px — inline with text (status, label)
20px — button icon, nav label icon
24px — standalone icon (empty state support, list row)
32px — feature icon (card header)
48px — empty state hero icon
```

**Color rules:**
```
Icon on light bg:       warm-500 (default), emerald-900 (active/primary)
Icon on emerald bg:     cream-200 (default), gold-500 (active/accent)
Icon in button:         inherits button text color
SOS icon:               white on danger-600
```

**Key icons used:**
```
ti-users          — Pilgrim list, group
ti-home           — Home / dashboard
ti-calendar       — Schedule, movement
ti-map-pin        — Location, hotel, bus
ti-alert-triangle — SOS, warning
ti-messages       — Chat
ti-device-sim     — Digital product / roaming
ti-file-export    — Export manifest
ti-qrcode         — QR scan / check-in
ti-crown          — Agent tier gold/silver
ti-wallet         — Commission, payout
ti-circle-check   — Present, confirmed, active
ti-clock          — Pending, scheduled
ti-user-check     — Check-in confirmed
ti-bus            — Transport / vehicle
ti-building       — Hotel / accommodation
ti-moon           — Umrah (optional decorative)
```

---

## 13. Safrat-Specific Design Patterns

### Gold ornament divider
Used to separate major sections within a card or page header. Not a plain `<hr>`.
```css
.gold-divider {
  height: 1px;
  background: linear-gradient(90deg, transparent 0%, #c9a84c 40%, #c9a84c 60%, transparent 100%);
  margin: 16px 0;
  opacity: 0.6;
}
```

### Serif + sans pairing
Every screen title uses Playfair Display serif. Body content immediately below uses Plus Jakarta Sans. This pairing signals the luxury positioning without being heavy-handed.

```tsx
// Pattern used throughout
<h1 className="font-serif text-2xl font-medium text-emerald-900">
  Daftar Jamaah
</h1>
<p className="font-sans text-sm text-warm-400 mt-1">
  248 jamaah · Musim Haji 1447H
</p>
```

### SOS visual treatment
SOS is a life-safety feature. Its design must never be ambiguous.
- Color: always danger-600 (#dc2626) — no variants, no "softer" version
- Shape: full-width rectangle (not round — round buttons feel casual)
- Text: uppercase, letter-spacing 0.04em, 16px minimum
- On Pilgrim App home: occupies bottom 25% of screen, always visible
- On Operator Dashboard: persistent alert banner top of viewport when any SOS is active

### Parchment texture concept
Background surfaces use `#fdf9f0` (cream-100) — warm, slightly off-white, reminiscent of premium paper. This is not a texture image — it's a color token. Never use pure white (`#ffffff`) as the main page background.

### Offline indicator
```tsx
// Shown when PowerSync detects no network (Group Leader + Pilgrim App)
<View className="bg-cream-300 py-2 px-4 flex-row items-center gap-2">
  <Icon name="ti-wifi-off" size={14} color="warm-500" />
  <Text className="text-warm-500 text-xs">
    Mode offline — data terakhir disinkronkan 5 menit lalu
  </Text>
</View>
```

---

## 14. Do / Don't

| Do | Don't |
|---|---|
| Serif for headings | Serif for body text (unreadable long-form) |
| Gold as accent only | Gold as primary background |
| Cream-100 as page bg | Pure white (#fff) as page bg |
| 48px tap targets | Anything smaller on mobile |
| Logical CSS (`ms-`, `me-`) | `margin-left/right` hardcoded |
| `body-lg` (17px) on Pilgrim App | Anything below 16px on Pilgrim App |
| SOS always visible | SOS behind a menu or scroll |
| Full-width buttons on mobile | Small inline buttons for primary CTA |
| Error text below input | Error tooltip on hover |
| Max 2 primary buttons per screen | 3+ gold buttons on same screen |
| Empty state with CTA | "No data found" alone |

---

*Safrat UI Spec — v1.0 — August 2026*
*Paired with: `CODEX_SPEC.md` (technical), `Hajj_Umrah_SaaS_PRD.docx` (product)*
