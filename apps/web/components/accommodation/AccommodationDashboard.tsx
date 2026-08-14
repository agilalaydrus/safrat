"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { IconBed, IconBuilding, IconPlus } from "@tabler/icons-react";
import { Hotel, Room } from "@hajj-saas/proto-gen/hajj/v1/accommodation_pb";
import { accommodationClient, seasonClient } from "@/lib/rpc";
import HotelFormDialog from "./HotelFormDialog";

type Summary = { rooms: number; occupied: number; capacity: number };
const cities = ["makkah", "madinah", "jeddah", "mina", "arafah"];

export default function AccommodationDashboard() {
  const [seasonId, setSeasonId] = useState("");
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [hotels, setHotels] = useState<Hotel[]>([]);
  const [summaries, setSummaries] = useState<Record<string, Summary>>({});
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [notice, setNotice] = useState("");
  const [search, setSearch] = useState("");

  useEffect(() => {
    seasonClient.listSeasons({})
      .then((response) => {
        setSeasons(response.seasons);
        setSeasonId(response.seasons.find((season) => season.isActive)?.id ?? response.seasons[0]?.id ?? "");
      })
      .catch(() => setNotice("Unable to load seasons."));
  }, []);

  const refresh = async () => {
    if (!seasonId) {
      setHotels([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const response = await accommodationClient.listHotels({ seasonId });
      setHotels(response.hotels);
      const pairs = await Promise.all(response.hotels.map(async (hotel) => [hotel.id, await accommodationClient.listRooms({ hotelId: hotel.id })] as const));
      const next: Record<string, Summary> = {};
      for (const [hotelId, rooms] of pairs) next[hotelId] = roomSummary(rooms.rooms);
      setSummaries(next);
    } catch {
      setNotice("Unable to load hotels.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void refresh(); }, [seasonId]);

  const grouped = useMemo(() => {
    const matchingHotels = hotels.filter((hotel) =>
      hotel.name.toLowerCase().includes(search.toLowerCase()) || hotel.city.toLowerCase().includes(search.toLowerCase()),
    );
    return cities
      .map((city) => ({ city, hotels: matchingHotels.filter((hotel) => hotel.city.toLowerCase() === city) }))
      .filter((group) => group.hotels.length);
  }, [hotels, search]);

  return (
    <main style={page}>
      <header style={header}>
        <div>
          <p style={eyebrow}>OPERATIONS / ACCOMMODATION</p>
          <h1 style={title}>Accommodation</h1>
          <p style={{ margin: 0, color: "var(--color-warm-500)" }}>Plan room inventory and pilgrim allocations for every stop.</p>
        </div>
        <div style={actions}>
          <select aria-label="Season" value={seasonId} onChange={(event) => setSeasonId(event.target.value)} style={select}>
            {seasons.length ? seasons.map((season) => <option key={season.id} value={season.id}>{season.name}{season.isActive ? " · Active" : ""}</option>) : <option value="">Select season</option>}
          </select>
          <button disabled={!seasonId} onClick={() => setFormOpen(true)} style={emerald}><IconPlus size={18} />Add Hotel</button>
        </div>
      </header>
      <div className="gold-divider" />
      {!loading && hotels.length > 0 && (
        <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search hotels..." style={searchInput} />
      )}
      {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
      {loading ? <section style={empty}>Loading accommodation...</section> : grouped.length ? (
        <div style={{ display: "grid", gap: 32 }}>
          {grouped.map(({ city, hotels: cityHotels }) => (
            <section key={city}>
              <div style={cityHeader}><p style={sectionEyebrow}>{city.toUpperCase()}</p><h2 style={{ margin: 0 }}>{city.charAt(0).toUpperCase() + city.slice(1)}</h2></div>
              <div style={grid}>{cityHotels.map((hotel) => <HotelCard key={hotel.id} hotel={hotel} summary={summaries[hotel.id]} />)}</div>
            </section>
          ))}
        </div>
      ) : <Empty onAdd={() => setFormOpen(true)} />}
      <HotelFormDialog open={formOpen} seasonId={seasonId} onClose={() => setFormOpen(false)} onSaved={() => { setNotice("Hotel added"); void refresh(); }} />
    </main>
  );
}

function HotelCard({ hotel, summary }: { hotel: Hotel; summary?: Summary }) {
  const dates = [formatDate(hotel.checkInDate), formatDate(hotel.checkOutDate)].filter(Boolean).join(" - ");
  return <article style={hotelCard}><div style={{ display: "flex", justifyContent: "space-between", gap: 12 }}><div><p style={sectionEyebrow}>{hotel.city}</p><h3 style={{ margin: 0, fontSize: 23 }}>{hotel.name}</h3></div>{hotel.starRating > 0 && <span style={star}>{"★".repeat(hotel.starRating)}</span>}</div><div className="gold-divider" /><p style={detail}>{dates || "Dates not yet set"}</p><div style={metrics}><span><IconBed size={17} />{summary?.rooms ?? 0} rooms</span><span>{summary?.occupied ?? 0}/{summary?.capacity ?? 0} occupied</span></div><Link href={`/dashboard/accommodation/${hotel.id}`} style={manage}>Manage rooms</Link></article>;
}

function Empty({ onAdd }: { onAdd: () => void }) {
  return <section style={empty}><IconBuilding size={48} color="var(--color-warm-400)" /><h2 style={{ margin: 0 }}>No hotels yet for this season</h2><p style={{ margin: 0, color: "var(--color-warm-500)", maxWidth: 380 }}>Add your first hotel to start building rooms and allocating pilgrims.</p><button onClick={onAdd} style={gold}><IconPlus size={18} />Add Hotel</button></section>;
}

function roomSummary(rooms: Room[]): Summary {
  return { rooms: rooms.length, occupied: rooms.reduce((total, room) => total + room.allocatedCount, 0), capacity: rooms.reduce((total, room) => total + room.capacity, 0) };
}

function formatDate(value: Hotel["checkInDate"]) {
  return value ? value.toDate().toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" }) : "";
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 24, flexWrap: "wrap" };
const actions: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center", background: "var(--color-emerald-900)", color: "var(--color-cream-100)", fontWeight: 700 };
const gold: React.CSSProperties = { ...emerald, background: "var(--color-gold-500)", color: "var(--color-warm-900)" };
const searchInput: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)", width: "100%", maxWidth: 360, margin: "16px 0 0" };
const cityHeader: React.CSSProperties = { marginBottom: 16 };
const sectionEyebrow: React.CSSProperties = { margin: "0 0 5px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", textTransform: "uppercase" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))", gap: 16 };
const hotelCard: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, display: "grid", gap: 12 };
const star: React.CSSProperties = { color: "var(--color-gold-600)", fontSize: 15, letterSpacing: 2 };
const detail: React.CSSProperties = { margin: 0, color: "var(--color-warm-500)", fontSize: 14 };
const metrics: React.CSSProperties = { display: "flex", gap: 16, flexWrap: "wrap", color: "var(--color-warm-500)", fontSize: 14 };
const manage: React.CSSProperties = { minHeight: 48, borderRadius: 8, display: "inline-flex", alignItems: "center", justifyContent: "center", background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", textDecoration: "none", fontWeight: 700, padding: "0 16px" };
const empty: React.CSSProperties = { minHeight: 320, display: "grid", placeItems: "center", alignContent: "center", gap: 12, textAlign: "center", padding: 24, border: "1px dashed var(--color-cream-400)", borderRadius: 12, background: "var(--color-cream-100)" };
