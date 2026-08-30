# Angka desain terukur — admmeeqt vs TawafiqHub

Dihitung dari bundle, bukan perkiraan. Dipakai sebagai dasar §2b DESAIN-DASHBOARD-ADMIN.md.

## rounded-* (total 1213)
```
xl×498, full×326, 2xl×173, lg×168, md×21, 3xl×10, sm×10, tl×3, tr×1, l×1, r×1, b×1
```

## shadow-* (total 188)
```
lift×69, soft×44, sm×28, glow×20, md×9, lg×8, color×4, brand-600×3, xl×2, slate-900×1
```

## transition-* (total 400)
```
all×215, colors×156, transform×18, opacity×11
```

## duration-* (total 14)
```
700×8, 300×2, 500×2, 150×1, 200×1
```

## animate-* (total 130)
```
fade-up×88, fade-in×18, ping×10, pulse×6, scale-in×6, spin×2
```

## ring-* (total 228)
```
brand-100×62, 4×54, 2×37, 1×19, white×14, white/10×10, white/15×6, brand-200×4, red-100×3, offset-1×2, white/25×2, white/40×1, amber-400×1, sky-200×1, violet-200×1, amber-200×1, emerald-200×1, slate-200×1, emerald-100×1, violet-100×1
```

## hover:* (total 474)
```
border-brand-300×69, text-brand-700×48, border-brand-200×46, bg-brand-50×40, text-brand-600×20, bg-white×17, bg-brand-25×16, bg-slate-50×14, bg-slate-100×13, text-red-600×12, opacity-100×12, brightness-110×11, text-ink×11, text-slate-700×10, text-white×10, text-slate-600×8, bg-brand-100×7, gap-2×7, text-red-500×7, translate-x-0×7, bg-red-50×7, text-brand-500×7, scale-110×6, text-brand-800×5
```

## gradient from/via/to (total 9)
```
brand-600×2, brand-500×2, gold-200×1, gold-400×1, red-50×1, amber-50×1, brand-50/70×1
```

## blur (total 36)
```
blur-3xl×18, backdrop-blur-sm×9, blur-2xl×8, backdrop-blur-md×1
```

## Token semantik (prop kelas satu)

**`tone`** — `green`×115, `blue`×102, `gold`×84, `amber`×73, `red`×65, `gray`×44, `sky`×32, `navy`×30, `purple`×15, `danger`×1

**`variant`** — `ghost`×112, `outline`×106, `secondary`×34, `gold`×11, `primary`×9, `dangerSoft`×5, `danger`×3

**`unit`** — `pcs`×19, `set`×12, `pax`×12, `seat`×6, `room-night`×4, `pax/hari`×4, `paket`×4, `bus`×2, `trip`×2, `box`×2, `kamar`×1, `rombongan`×1, `pesan/bulan`×1, `transaksi/bulan`×1

**`stage`** — `baru`×9, `kontak`×7, `penawaran`×7, `hot`×7, `closing`×6, `batal`×5

**`priority`** — `normal`×6, `penting`×4, `high`×3, `medium`×2, `low`×2, `critical`×1, `kritis`×1

## TawafiqHub saat ini

```
rounded-* : 2      shadow-* : 0     transition-* : 0
animate-* : 0      ring-*   : 0     hover:*      : 0
```

Penyebab: `Button.tsx`, `StatCard.tsx`, `PageHero.tsx` memakai inline style object,
yang tidak bisa menyatakan `:hover` / `:focus-visible` / `:active`.
