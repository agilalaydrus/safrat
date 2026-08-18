import { SeasonType } from "@hajj-saas/proto-gen/hajj/v1/season_pb";

/** Single source of truth for season-type labels — shared by the create/edit dialog and the list. */
export const SEASON_TYPE_OPTIONS: { value: SeasonType; label: string }[] = [
  { value: SeasonType.HAJJ, label: "Haji" },
  { value: SeasonType.UMRAH_REGULER, label: "Umrah Reguler" },
  { value: SeasonType.UMRAH_RAJAB, label: "Umrah Rajab" },
  { value: SeasonType.UMRAH_RAMADHAN, label: "Umrah Ramadhan" },
  { value: SeasonType.UMRAH_SYAWAL, label: "Umrah Syawal" },
  { value: SeasonType.UMRAH_DZULQAIDAH, label: "Umrah Dzulqa'dah" },
];

export const SEASON_TYPE_LABEL: Record<number, string> = Object.fromEntries(SEASON_TYPE_OPTIONS.map((o) => [o.value, o.label]));
