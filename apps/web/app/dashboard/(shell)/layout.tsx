"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  IconBuildingHospital,
  IconBuildingCommunity,
  IconBus,
  IconCalendar,
  IconCalendarEvent,
  IconCash,
  IconChecklist,
  IconChevronRight,
  IconClipboardList,
  IconChartFunnel,
  IconFileAnalytics,
  IconFiles,
  IconHeartHandshake,
  IconHome,
  IconLayoutDashboard,
  IconLock,
  IconLogout,
  IconMapPinExclamation,
  IconMenu2,
  IconPackage,
  IconPlane,
  IconRadar,
  IconReceipt2,
  IconSchool,
  IconWallet,
  IconSettings,
  IconShieldCheck,
  IconShoppingCart,
  IconTruckDelivery,
  IconSos,
  IconSpeakerphone,
  IconUserCheck,
  IconUserDollar,
  IconUsers,
  IconUsersGroup,
  IconX,
} from "@tabler/icons-react";
import { authClient } from "@/lib/auth-client";
import { operatorClient, seasonClient, subscriptionClient } from "@/lib/rpc";
import { RequireAccess } from "@/components/auth/RequireAccess";
import { RequireTwoFactor } from "@/components/auth/RequireTwoFactor";
import { invalidateMyAccessCache } from "@/lib/access-cache";

type NavItem = readonly [label: string, href: string, icon: typeof IconLayoutDashboard, feature?: "branches" | "crm"];
type NavGroup = { readonly label: string; readonly items: readonly NavItem[] };

const nav: readonly NavGroup[] = [
  { label: "", items: [["Ringkasan", "/dashboard", IconLayoutDashboard]] },
  {
    label: "Jamaah",
    items: [
      ["Musim", "/dashboard/seasons", IconCalendar],
      ["Jamaah", "/dashboard/pilgrims", IconUsers],
      ["Dokumen", "/dashboard/documents", IconFiles],
      ["Pendaftaran", "/dashboard/registrations", IconClipboardList],
      ["Grup", "/dashboard/groups", IconUsersGroup],
      ["Muttawwif", "/dashboard/muttawwif", IconUserCheck],
      ["Kloter", "/dashboard/kloter", IconPlane],
      ["Asuransi", "/dashboard/insurance", IconShieldCheck],
    ],
  },
  {
    label: "Operasional",
    items: [
      ["Monitoring", "/dashboard/monitoring", IconRadar],
      ["Akomodasi", "/dashboard/accommodation", IconBuildingHospital],
      ["Transportasi", "/dashboard/transport", IconBus],
      ["Komunikasi", "/dashboard/communication", IconSpeakerphone],
      ["SOS", "/dashboard/sos", IconSos],
      ["Jamaah Terpisah", "/dashboard/lost", IconMapPinExclamation],
      ["Manasik", "/dashboard/manasik", IconSchool],
      ["Jadwal Tim", "/dashboard/schedule", IconChecklist],
      ["Jadwal Saya", "/dashboard/my-schedule", IconCalendarEvent],
    ],
  },
  {
    label: "Bisnis",
    items: [
      ["CRM Leads", "/dashboard/crm", IconChartFunnel, "crm"],
      ["Arus Kas", "/dashboard/cashflow", IconCash],
      ["Vendor", "/dashboard/vendors", IconHeartHandshake],
      ["Inventaris", "/dashboard/inventaris", IconPackage],
      ["Produk", "/dashboard/products", IconShoppingCart],
      ["Pengiriman", "/dashboard/shipments", IconTruckDelivery],
      ["Pesanan", "/dashboard/orders", IconReceipt2],
      ["Refund & Saldo", "/dashboard/refunds", IconWallet],
      ["Tour Leader", "/dashboard/agents", IconUserDollar],
      ["Cabang", "/dashboard/cabang", IconBuildingCommunity, "branches"],
    ],
  },
  {
    label: "Laporan",
    items: [["Laporan & Analitik", "/dashboard/reports", IconFileAnalytics]],
  },
];

function isActiveRoute(pathname: string, href: string) {
  return pathname === href || (href !== "/dashboard" && pathname.startsWith(href));
}

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { data: session } = authClient.useSession();
  const [open, setOpen] = useState(false);
  const [operator, setOperator] = useState("");
  const [season, setSeason] = useState("");
  const [branchesEnabled, setBranchesEnabled] = useState<boolean>();
  const [crmEnabled, setCRMEnabled] = useState<boolean>();

  useEffect(() => {
    operatorClient.getMyOperator({}).then((value) => setOperator(value.name)).catch(async (err) => {
      if (ConnectError.from(err).code === Code.FailedPrecondition) {
        router.push("/dashboard/langganan");
        return;
      }
      if (err?.code === "unauthenticated") {
        await authClient.getSession({ fetchOptions: { cache: "no-store" } });
        operatorClient.getMyOperator({}).then((value) => setOperator(value.name)).catch(() => router.push("/sign-in"));
      }
    });
    seasonClient.listSeasons({}).then((value) => {
      setSeason((value.seasons.find((item) => item.isActive) ?? value.seasons[0])?.name ?? "");
    }).catch(async (err) => {
      if (ConnectError.from(err).code === Code.FailedPrecondition) {
        router.push("/dashboard/langganan");
        return;
      }
      if (err?.code === "unauthenticated") {
        await authClient.getSession({ fetchOptions: { cache: "no-store" } });
        seasonClient.listSeasons({}).then((value) => {
          setSeason((value.seasons.find((item) => item.isActive) ?? value.seasons[0])?.name ?? "");
        }).catch(() => router.push("/sign-in"));
      }
    });
  }, [router]);

  useEffect(() => setOpen(false), [pathname]);

  useEffect(() => {
    subscriptionClient.getMySubscription({}).then((value) => {
      setBranchesEnabled(value.entitlement?.branchesEnabled ?? false);
      setCRMEnabled(value.entitlement?.crmEnabled ?? false);
    }).catch(() => { setBranchesEnabled(false); setCRMEnabled(false); });
  }, []);

  async function signOut() {
    await authClient.signOut();
    invalidateMyAccessCache();
    router.push("/sign-in");
  }

  const currentLabel = useMemo(() => {
    if (pathname.startsWith("/dashboard/settings")) return "Pengaturan";
    return nav.flatMap((group) => group.items)
      .find(([, href]) => isActiveRoute(pathname, href))?.[0] ?? "Dashboard";
  }, [pathname]);

  const userLabel = session?.user?.name || session?.user?.email?.split("@")[0] || "Pengguna";
  const initials = (operator || userLabel).split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]).join("").toUpperCase();
  const settingsActive = pathname.startsWith("/dashboard/settings");

  const sidebarNode = (
    <nav className="dashboard-sidebar" aria-label="Navigasi operator">
      <div className="dashboard-brand-block">
        <div className="dashboard-brand-row">
          <Link href="/" aria-label="TawafiqHub home" className="dashboard-brand">
            <span className="dashboard-brand-mark" aria-hidden>TH</span>
            <span>TawafiqHub</span>
          </Link>
          <button type="button" className="dashboard-drawer-close" aria-label="Tutup menu" onClick={() => setOpen(false)}>
            <IconX size={20} />
          </button>
        </div>
        {operator && <p className="dashboard-operator-name">{operator}</p>}
        {season && <p className="dashboard-season-name">{season}</p>}
      </div>

      <div className="dashboard-nav-scroll">
        {nav.map((group, groupIndex) => (
          <section className="dashboard-nav-group" key={group.label || groupIndex} aria-label={group.label || "Utama"}>
            {group.label && <p className="dashboard-nav-label">{group.label}</p>}
            <ul className="dashboard-nav-list">
              {group.items.map(([label, href, Icon, feature]) => {
                const active = isActiveRoute(pathname, href);
                const locked = feature === "branches" && branchesEnabled === false;
                const featureLocked = locked || (feature === "crm" && crmEnabled === false);
                return (
                  <li key={href}>
                    {featureLocked ? <button type="button" className="dashboard-nav-item is-locked" onClick={() => router.push("/dashboard/langganan")} aria-label={`${label} tersedia mulai paket Growth`}>
                      <span className="dashboard-nav-icon"><Icon size={18} stroke={1.8} aria-hidden /></span>
                      <span>{label}</span><IconLock size={14} aria-hidden />
                    </button> : <Link className={`dashboard-nav-item${active ? " is-active" : ""}`} href={href} aria-current={active ? "page" : undefined}>
                      <span className="dashboard-nav-icon"><Icon size={18} stroke={1.8} aria-hidden /></span>
                      <span>{label}</span>
                    </Link>}
                  </li>
                );
              })}
            </ul>
          </section>
        ))}
      </div>

      <div className="dashboard-sidebar-footer">
        <Link className={`dashboard-nav-item${settingsActive ? " is-active" : ""}`} href="/dashboard/settings" aria-current={settingsActive ? "page" : undefined}>
          <span className="dashboard-nav-icon"><IconSettings size={18} stroke={1.8} aria-hidden /></span>
          <span>Pengaturan</span>
        </Link>
        <div className="dashboard-user-card">
          <span className="dashboard-avatar" aria-hidden>{initials || "TH"}</span>
          <span className="dashboard-user-copy">
            <strong>{userLabel}</strong>
            <small>{session?.user?.email}</small>
          </span>
          <button type="button" className="dashboard-signout" onClick={() => void signOut()} aria-label="Keluar dari akun">
            <IconLogout size={18} stroke={1.8} />
          </button>
        </div>
      </div>
    </nav>
  );

  return (
    <RequireAccess role="staff">
      <RequireTwoFactor mode="enforce">
        <div className="dashboard-shell">
          <aside data-desktop-sidebar className="dashboard-sidebar-rail no-print">{sidebarNode}</aside>
          {open && (
            <div className="dashboard-drawer-backdrop no-print" role="presentation" onClick={() => setOpen(false)}>
              <div className="dashboard-drawer" onClick={(event) => event.stopPropagation()}>{sidebarNode}</div>
            </div>
          )}
          <div className="dashboard-content">
            <header className="dashboard-topbar no-print">
              <button type="button" className="dashboard-menu-button" aria-label="Buka menu" aria-expanded={open} onClick={() => setOpen(true)}>
                <IconMenu2 size={21} stroke={1.8} />
              </button>
              <div className="dashboard-breadcrumb" aria-label="Lokasi halaman">
                <IconHome size={16} stroke={1.8} aria-hidden />
                <IconChevronRight size={15} stroke={1.8} aria-hidden />
                <span>{currentLabel}</span>
              </div>
              <div className="dashboard-topbar-user">
                <span>Hai, {userLabel}</span>
                <span className="dashboard-avatar" aria-hidden>{initials || "TH"}</span>
              </div>
            </header>
            <main className="dashboard-workspace">{children}</main>
          </div>
        </div>
      </RequireTwoFactor>
    </RequireAccess>
  );
}
