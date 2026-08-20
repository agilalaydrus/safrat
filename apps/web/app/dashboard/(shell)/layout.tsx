"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { IconBuildingHospital, IconBus, IconCalendar, IconCalendarEvent, IconCash, IconChartBar, IconChecklist, IconClipboardList, IconClock, IconFileAnalytics, IconFiles, IconHeartHandshake, IconLayoutDashboard, IconLogout, IconMapPinExclamation, IconMenu2, IconPlane, IconReceipt2, IconSettings, IconShieldCheck, IconShoppingCart, IconSos, IconSpeakerphone, IconUserDollar, IconUsers, IconUsersGroup } from "@tabler/icons-react";
import { authClient } from "@/lib/auth-client";
import { operatorClient, seasonClient } from "@/lib/rpc";
import { RequireAccess } from "@/components/auth/RequireAccess";
import { invalidateMyAccessCache } from "@/lib/access-cache";

const nav = [
  { label: "", items: [["Ringkasan", "/dashboard", IconLayoutDashboard]] },
  {
    label: "Jamaah",
    items: [
      ["Musim", "/dashboard/seasons", IconCalendar],
      ["Jamaah", "/dashboard/pilgrims", IconUsers],
      ["Dokumen", "/dashboard/documents", IconFiles],
      ["Pendaftaran", "/dashboard/registrations", IconClipboardList],
      ["Daftar Tunggu", "/dashboard/waitlist", IconClock],
      ["Rombongan", "/dashboard/groups", IconUsersGroup],
      ["Kloter", "/dashboard/kloter", IconPlane],
      ["Asuransi", "/dashboard/insurance", IconShieldCheck],
    ],
  },
  {
    label: "Operasional",
    items: [
      ["Akomodasi", "/dashboard/accommodation", IconBuildingHospital],
      ["Transportasi", "/dashboard/transport", IconBus],
      ["Komunikasi", "/dashboard/communication", IconSpeakerphone],
      ["SOS", "/dashboard/sos", IconSos],
      ["Jamaah Terpisah", "/dashboard/lost", IconMapPinExclamation],
      ["Jadwal Tim", "/dashboard/schedule", IconChecklist],
      ["Jadwal Saya", "/dashboard/my-schedule", IconCalendarEvent],
    ],
  },
  {
    label: "Bisnis",
    items: [
      ["Arus Kas", "/dashboard/cashflow", IconCash],
      ["Vendor", "/dashboard/vendors", IconHeartHandshake],
      ["Produk", "/dashboard/products", IconShoppingCart],
      ["Pesanan", "/dashboard/orders", IconReceipt2],
      ["Tour Leader", "/dashboard/agents", IconUserDollar],
    ],
  },
  {
    label: "Laporan",
    items: [
      ["Laporan", "/dashboard/reports", IconFileAnalytics],
      ["Analitik", "/dashboard/analytics", IconChartBar],
    ],
  },
] as const;

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname(); const router = useRouter(); const { data: session } = authClient.useSession(); const [open, setOpen] = useState(false); const [operator, setOperator] = useState(""); const [season, setSeason] = useState("");
  useEffect(() => { operatorClient.getMyOperator({}).then((value) => setOperator(value.name)).catch(async (err) => { if (err?.code === "unauthenticated") { await authClient.getSession({ fetchOptions: { cache: "no-store" } }); operatorClient.getMyOperator({}).then((value) => setOperator(value.name)).catch(() => router.push("/sign-in")); } }); seasonClient.listSeasons({}).then((value) => setSeason((value.seasons.find((item) => item.isActive) ?? value.seasons[0])?.name ?? "")).catch(async (err) => { if (err?.code === "unauthenticated") { await authClient.getSession({ fetchOptions: { cache: "no-store" } }); seasonClient.listSeasons({}).then((value) => setSeason((value.seasons.find((item) => item.isActive) ?? value.seasons[0])?.name ?? "")).catch(() => router.push("/sign-in")); } }); }, [router]);
  async function signOut() { await authClient.signOut(); invalidateMyAccessCache(); router.push("/sign-in"); }
  const settingsActive = pathname.startsWith("/dashboard/settings");
  const sidebarNode = <nav style={sidebarStyle}><div style={brand}><Link href="/" aria-label="Safrat home" style={logo}>Safrat</Link>{operator && <p style={org}>{operator}</p>}{season && <p style={seasonStyle}><i style={dot} />{season}</p>}</div><div className="gold-divider" style={{ margin: "0 16px 8px" }} /><div style={list}>{nav.map((group, groupIndex) => <div key={group.label || groupIndex} style={groupIndex > 0 ? groupBlock : undefined}>{group.label && <p style={groupLabel}>{group.label}</p>}<ul style={groupList}>{group.items.map(([label, href, Icon]) => { const active = pathname === href || (href !== "/dashboard" && pathname.startsWith(href)); return <li key={href}><Link href={href} onClick={() => setOpen(false)} style={{ ...item, ...(active ? activeItem : {}) }}><Icon size={18} />{label}{active && <i style={activeDot} />}</Link></li>; })}</ul></div>)}</div><div style={bottom}><div className="gold-divider" /><Link href="/dashboard/settings" onClick={() => setOpen(false)} style={{ ...item, padding: "12px 0", ...(settingsActive ? activeItem : {}), ...(settingsActive ? { padding: "12px 0 12px 3px" } : {}) }}><IconSettings size={18} />Pengaturan{settingsActive && <i style={activeDot} />}</Link><p style={email}>{session?.user?.email}</p><button className="btn-signout" onClick={() => void signOut()} style={signOutStyle}><IconLogout size={16} />Keluar</button></div></nav>;
  return <RequireAccess role="staff"><div style={shell}><aside data-desktop-sidebar className="no-print" style={desktop}>{sidebarNode}</aside>{open && <div style={overlay} onClick={() => setOpen(false)}><div style={mobile} onClick={(event) => event.stopPropagation()}>{sidebarNode}</div></div>}<div style={content}><header data-mobile-header="" className="no-print" style={mobileHeader}><button aria-label="Open menu" onClick={() => setOpen(true)} style={menu}><IconMenu2 size={22} /></button><div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 1 }}><Link href="/" onClick={() => setOpen(false)} aria-label="Safrat home" style={{ ...logo, fontSize: 18, lineHeight: 1 }}>Safrat</Link>{season && <span style={{ fontSize: 10, color: "rgba(255,255,255,0.55)", letterSpacing: "0.06em", fontFamily: "'Plus Jakarta Sans', sans-serif", fontWeight: 500 }}>{season}</span>}</div><div style={{ width: 40 }} /></header>{children}</div></div></RequireAccess>;
}
const width=240; const shell:React.CSSProperties={display:"flex",minHeight:"100vh"}; const sidebarStyle:React.CSSProperties={width,minHeight:"100vh",background:"var(--color-emerald-900)",display:"flex",flexDirection:"column",paddingTop:24}; const desktop:React.CSSProperties={width,flexShrink:0,position:"sticky",top:0,height:"100vh",overflowY:"auto",display:"none"}; const content:React.CSSProperties={flex:1,minWidth:0}; const overlay:React.CSSProperties={position:"fixed",inset:0,zIndex:50,background:"rgba(0,0,0,.5)"}; const mobile:React.CSSProperties={width,height:"100vh",overflowY:"auto"}; const mobileHeader: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 16px", background: "rgba(13,61,39,0.92)", backdropFilter: "blur(12px)", WebkitBackdropFilter: "blur(12px)", position: "sticky", top: 0, zIndex: 40, borderBottom: "1px solid rgba(201,168,76,0.20)" }; const menu: React.CSSProperties = { width: 40, height: 40, border: "1px solid rgba(201,168,76,0.25)", borderRadius: 10, background: "rgba(255,255,255,0.06)", color: "var(--color-cream-100)", display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0 }; const brand:React.CSSProperties={padding:"0 20px 16px"}; const logo:React.CSSProperties={fontFamily:"'Playfair Display', Georgia, serif",color:"var(--color-gold-500)",fontSize:28,fontWeight:700}; const org:React.CSSProperties={color:"var(--color-cream-100)",fontSize:13,margin:"8px 0 4px",fontWeight:600}; const seasonStyle:React.CSSProperties={margin:0,color:"rgba(255,255,255,.6)",fontSize:12,display:"flex",alignItems:"center",gap:6}; const dot:React.CSSProperties={width:6,height:6,borderRadius:"50%",background:"var(--color-gold-500)"}; const list:React.CSSProperties={margin:0,padding:0,flex:1}; const groupBlock:React.CSSProperties={marginTop:18}; const groupLabel:React.CSSProperties={margin:"0 0 4px",padding:"0 20px",fontSize:11,fontWeight:700,letterSpacing:".08em",color:"rgba(255,255,255,.35)",textTransform:"uppercase"}; const groupList:React.CSSProperties={listStyle:"none",margin:0,padding:0}; const item:React.CSSProperties={minHeight:48,display:"flex",alignItems:"center",gap:12,padding:"12px 20px",color:"rgba(255,255,255,.7)",fontSize:14,position:"relative"}; const activeItem:React.CSSProperties={color:"var(--color-cream-100)",fontWeight:700,background:"rgba(255,255,255,.08)",borderLeft:"3px solid var(--color-gold-500)",padding:"12px 20px 12px 17px"}; const activeDot:React.CSSProperties={width:6,height:6,borderRadius:"50%",background:"var(--color-gold-500)",marginLeft:"auto"}; const bottom:React.CSSProperties={padding:"8px 20px 24px"}; const email:React.CSSProperties={color:"rgba(255,255,255,.5)",fontSize:12,margin:"16px 0 8px"}; const signOutStyle:React.CSSProperties={minHeight:40,border:0,background:"transparent",color:"rgba(255,255,255,.5)",display:"flex",alignItems:"center",gap:8};
