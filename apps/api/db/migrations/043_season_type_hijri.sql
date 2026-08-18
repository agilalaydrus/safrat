-- +goose Up
-- Musim di industri umrah Indonesia dijual per periode Hijriah, bukan cuma
-- "Haji"/"Umrah" datar — Rajab, Ramadhan, Syawal, dan Dzulqa'dah masing-
-- masing punya demand dan harga sendiri. HAJJ tetap satu nilai (Dzulhijjah,
-- penawaran terpisah); UMRAH pecah jadi lima. Sub-periode yang lebih halus
-- (mis. "Ramadhan Akhir/10 hari terakhir") dibedakan lewat nama musim +
-- start_date/end_date yang sudah ada, bukan nilai enum baru lagi.
CREATE TYPE season_type_new AS ENUM ('HAJJ', 'UMRAH_REGULER', 'UMRAH_RAJAB', 'UMRAH_RAMADHAN', 'UMRAH_SYAWAL', 'UMRAH_DZULQAIDAH');

ALTER TABLE seasons ALTER COLUMN type TYPE season_type_new USING (
  CASE type::text
    WHEN 'UMRAH' THEN 'UMRAH_REGULER'
    ELSE type::text
  END
)::season_type_new;

DROP TYPE season_type;
ALTER TYPE season_type_new RENAME TO season_type;

-- +goose Down
CREATE TYPE season_type_old AS ENUM ('HAJJ', 'UMRAH');
ALTER TABLE seasons ALTER COLUMN type TYPE season_type_old USING (
  CASE WHEN type::text = 'HAJJ' THEN 'HAJJ' ELSE 'UMRAH' END
)::season_type_old;
DROP TYPE season_type;
ALTER TYPE season_type_old RENAME TO season_type;
