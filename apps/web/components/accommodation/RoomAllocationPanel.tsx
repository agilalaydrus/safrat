"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { IconGenderFemale, IconGenderMale, IconSearch, IconUserMinus, IconUserPlus } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { Room, RoomManifest } from "@hajj-saas/proto-gen/hajj/v1/accommodation_pb";
import { accommodationClient, pilgrimClient } from "@/lib/rpc";
import { Button } from "@/components/ui/Button";
import { DetailDrawer } from "@/components/ui/DetailDrawer";
import { ProgressBar } from "@/components/ui/ProgressBar";

type Props = { open: boolean; room: Room | null; seasonId: string; allocatedPilgrimIds: Set<string>; onClose: () => void; onChanged: () => void };

export default function RoomAllocationPanel({ open, room, seasonId, allocatedPilgrimIds, onClose, onChanged }: Props) {
  const [manifest, setManifest] = useState<RoomManifest>(); const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]); const [query, setQuery] = useState(""); const [term, setTerm] = useState(""); const [notice, setNotice] = useState(""); const [workingId, setWorkingId] = useState(""); const [loading, setLoading] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);
  const roomId = room?.id;
  const refresh = useCallback((id = roomId) => { if (!id) return; accommodationClient.getRoomManifest({ roomId: id }).then(setManifest).catch(() => setNotice("Gagal memuat penghuni kamar.")); }, [roomId]);
  useEffect(() => {
    if (!open || !roomId) return;
    let cancelled = false;
    setNotice(""); setQuery(""); setManifest(undefined); setLoading(true);
    Promise.all([accommodationClient.getRoomManifest({ roomId }), listSeasonPilgrims(seasonId)])
      .then(([manifestValue, pilgrimValues]) => { if (!cancelled) { setManifest(manifestValue); setPilgrims(pilgrimValues); } })
      .catch(() => { if (!cancelled) setNotice("Data penempatan belum dapat dimuat. Coba tutup lalu buka kembali panel ini."); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [open, roomId, seasonId]);
  useEffect(() => { const timeout = window.setTimeout(() => setTerm(query), 300); return () => window.clearTimeout(timeout); }, [query]);
  const occupants = useMemo(() => manifest?.pilgrims ?? [], [manifest]); const full = !!manifest && manifest.availableCapacity <= 0;
  const isMahramSuggested = useCallback((pilgrim: Pilgrim) => occupants.some((occupant) => occupant.id === pilgrim.mahramId || pilgrim.id === occupant.mahramId), [occupants]);
  const mahramWarnings = useMemo(() => occupants.filter((occupant) => occupant.mahramId && !occupants.some((other) => other.id === occupant.mahramId)).map((occupant) => { const mahram = pilgrims.find((p) => p.id === occupant.mahramId); return { occupant, mahramName: mahram?.fullName ?? "pasangan mahram-nya" }; }), [occupants, pilgrims]);
  const candidates = useMemo(() => pilgrims
    .filter((pilgrim) => !allocatedPilgrimIds.has(pilgrim.id) && !pilgrim.isSubstituted && `${pilgrim.fullName} ${pilgrim.passportNumber}`.toLowerCase().includes(term.toLowerCase()))
    .sort((a, b) => Number(isMahramSuggested(b)) - Number(isMahramSuggested(a))), [allocatedPilgrimIds, pilgrims, term, isMahramSuggested]);
  if (!open || !room || !roomId) return null;
  const activeRoom = room;
  async function remove(pilgrimId: string) { setWorkingId(pilgrimId); setNotice(""); try { await accommodationClient.deallocatePilgrim({ pilgrimId, roomId }); refresh(); onChanged(); } catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal mengeluarkan jamaah."); } finally { setWorkingId(""); } }
  async function assign(pilgrimId: string) { setWorkingId(pilgrimId); setNotice(""); try { await accommodationClient.allocatePilgrim({ roomId, pilgrimId }); refresh(roomId); onChanged(); } catch (caught) { const message = caught instanceof Error ? caught.message : "Gagal menempatkan jamaah."; if (/capacity|resource_exhausted|full/i.test(message)) setNotice("Kamar sudah penuh"); else if (/designated|gender|failed_precondition/i.test(message)) setNotice(`Jenis kelamin tidak sesuai - kamar ini khusus ${activeRoom.gender}`); else setNotice(message); } finally { setWorkingId(""); } }
  const occupied = Math.max(0, room.capacity - (manifest?.availableCapacity ?? room.capacity));
  return <DetailDrawer open={open} onClose={onClose} title={`Kamar ${room.roomNumber}`} subtitle={`${ROOM_TYPE_LABEL[room.roomType] ?? label(room.roomType)} · ${label(room.gender)} · ${occupied}/${room.capacity} tempat tidur terisi`} closeLabel={`Tutup penempatan kamar ${room.roomNumber}`} initialFocusRef={searchRef} className="accommodation-allocation-drawer">
    <ProgressBar label="Okupansi kamar" value={occupied} max={room.capacity} valueLabel={`${occupied}/${room.capacity} tempat tidur`} tone={full ? "warning" : "success"} />
    {mahramWarnings.length > 0 && <div className="accommodation-allocation__warnings">{mahramWarnings.map(({ occupant, mahramName }) => <p key={occupant.id} role="status" className="accommodation-allocation__warning">Peringatan: {occupant.fullName} memiliki pasangan mahram ({mahramName}) yang tidak berada di kamar ini.</p>)}</div>}
    {notice && <p role="alert" className="accommodation-allocation__notice">{notice}</p>}
    <section className="accommodation-allocation__section" aria-labelledby="current-occupants-title">
      <h3 id="current-occupants-title">Penghuni dari cabang Anda</h3>
      {loading ? <p className="accommodation-allocation__muted">Memuat penghuni kamar…</p> : occupants.length ? occupants.map((pilgrim) => <div key={pilgrim.id} className="accommodation-allocation__person"><div><strong>{pilgrim.fullName}</strong><span>{pilgrim.passportNumber} · {pilgrim.gender === Gender.FEMALE ? "Wanita" : "Pria"}</span></div><Button variant="danger" size="sm" disabled={workingId === pilgrim.id} onClick={() => remove(pilgrim.id)}><IconUserMinus size={17} />{workingId === pilgrim.id ? "Mengeluarkan…" : "Keluarkan"}</Button></div>) : <p className="accommodation-allocation__muted">Belum ada jamaah dari cabang Anda di kamar ini.</p>}
      {!loading && occupied > occupants.length && <p className="accommodation-allocation__muted">{occupied - occupants.length} tempat tidur lain digunakan cabang lain. Identitas jamaahnya disembunyikan.</p>}
    </section>
    {!full && <section className="accommodation-allocation__section" aria-labelledby="assign-pilgrim-title"><h3 id="assign-pilgrim-title">Tempatkan jamaah</h3><label className="accommodation-allocation__search"><span className="sr-only">Cari jamaah yang belum ditempatkan</span><IconSearch size={18} aria-hidden="true" /><input ref={searchRef} value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Cari nama jamaah atau nomor paspor…" /></label>{!loading && candidates.slice(0, 20).map((pilgrim) => <div key={pilgrim.id} className="accommodation-allocation__person"><div><strong>{pilgrim.fullName}</strong>{isMahramSuggested(pilgrim) && <span className="accommodation-allocation__mahram">Disarankan · pasangan mahram</span>}<span>{pilgrim.passportNumber} · {pilgrim.gender === Gender.FEMALE ? <><IconGenderFemale size={15} /> Wanita</> : <><IconGenderMale size={15} /> Pria</>}</span></div><Button variant="emerald" size="sm" disabled={workingId === pilgrim.id} onClick={() => assign(pilgrim.id)}><IconUserPlus size={17} />{workingId === pilgrim.id ? "Menempatkan…" : "Tempatkan"}</Button></div>)}{!loading && !candidates.length && <p className="accommodation-allocation__muted">{query ? "Tidak ada jamaah yang cocok dengan pencarian ini." : "Semua jamaah yang tersedia sudah mendapat kamar di hotel ini."}</p>}</section>}
    {full && <p role="status" className="accommodation-allocation__full">Kamar sudah penuh. Keluarkan penghuni terlebih dahulu sebelum menempatkan jamaah lain.</p>}
  </DetailDrawer>;
}
async function listSeasonPilgrims(seasonId: string): Promise<Pilgrim[]> {
  const result: Pilgrim[] = [];
  let offset = 0;
  while (true) {
    const response = await pilgrimClient.listPilgrims({ seasonId, limit: 100, offset });
    result.push(...response.pilgrims);
    offset += response.pilgrims.length;
    if (!response.pilgrims.length || offset >= Number(response.totalCount)) return result;
  }
}
function label(value: string) { return value ? value.charAt(0).toUpperCase() + value.slice(1) : "-"; }
const ROOM_TYPE_LABEL: Record<string, string> = { single: "Single", double: "Double", triple: "Triple", quad: "Quadruple", quadruple: "Quadruple" };
