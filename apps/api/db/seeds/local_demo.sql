-- Local-only Maktour Indonesia demo fixture.
-- Re-running this script replaces only the deterministic demo operator and its cascaded data.
BEGIN;

DELETE FROM operators WHERE id = '00000000-0000-4000-8000-000000000001';

INSERT INTO operators (id, better_auth_org_id, name, country, email, license_number, plan)
SELECT
  '00000000-0000-4000-8000-000000000001',
  COALESCE(
    (SELECT s."activeOrganizationId" FROM session s WHERE s."activeOrganizationId" IS NOT NULL AND s."expiresAt" > NOW() ORDER BY s."expiresAt" DESC LIMIT 1),
    (SELECT m."organizationId" FROM member m ORDER BY m."createdAt" DESC LIMIT 1),
    'maktour'
  ),
  'Maktour Indonesia',
  'ID',
  'ops@maktour.co.id',
  'MAKTOUR-LOCAL',
  'PRO'
;

-- Dates are anchored to NOW() rather than hardcoded — re-running this seed
-- must always produce a current/upcoming season, never one that silently
-- drifts into the past as real time moves on. The active season starts a
-- few days out and runs ~16 days (matching a real Hajj season's length);
-- hotels/movements below reference this row's start_date directly so they
-- can never fall out of sync with it.
INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, is_active)
VALUES
  ('00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001',
    'Musim Haji ' || extract(year from NOW() + INTERVAL '5 days')::text, 'HAJJ',
    date_trunc('day', NOW()) + INTERVAL '5 days',
    date_trunc('day', NOW()) + INTERVAL '21 days' + INTERVAL '23 hours 59 minutes 59 seconds',
    true),
  ('00000000-0000-4000-8000-000000000102', '00000000-0000-4000-8000-000000000001',
    'Musim Haji ' || (extract(year from NOW())::int - 1)::text, 'HAJJ',
    date_trunc('day', NOW()) - INTERVAL '410 days',
    date_trunc('day', NOW()) - INTERVAL '390 days',
    false);

-- Two kloter (Kemenag departure batches), each anchored to the season's own
-- start_date the same way hotels/movements are — SOC-01 is the earlier
-- batch (departs on the season's actual start date), SOC-02 follows two
-- days later. GROUP-A's pilgrims fly in SOC-01, GROUP-B's in SOC-02, and
-- the four ungrouped pilgrims are also left without a kloter — demoing the
-- "belum ada kloter" state the same way they demo "belum ada rombongan".
INSERT INTO kloters (id, operator_id, season_id, code, embarkation, flight_number, departure_date, capacity)
SELECT '00000000-0000-4000-8000-000000001001'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, 'SOC-01', 'Soekarno-Hatta', 'SV 815', start_date, 45 FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000001002'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, 'SOC-02', 'Soekarno-Hatta', 'SV 823', start_date + INTERVAL '2 days', 45 FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101';

-- Three groups, each led by a real Muttawwif account (created once via
-- Better Auth's sign-up API + a member row, not by this script — a fixed
-- Better Auth user id, not "whoever is logged in", so the assignment is
-- stable across reseeds and every group always has a leader on both the
-- admin Groups page and the leader's own /leader app).
INSERT INTO groups (id, season_id, operator_id, name, capacity, leader_id)
VALUES
  ('00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', 'GROUP-A', 40, 'D862T2c4awEabjAJRNY2Cr1DC2MTfyLI'),
  ('00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', 'GROUP-B', 40, 'EomOhXSQ3HfcIMk5hOMKliraIcEaqQuo'),
  ('00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', 'GROUP-C', 40, '5bv99BUnYfpzIcbtyC2vcSPzlD0fgV6x');

-- Leaders automatically become agents (GroupService.UpdateGroup does this
-- for real assignments made through the app) — this seed inserts groups
-- directly, bypassing that service, so it's done explicitly here to keep
-- demo data consistent with the rule.
INSERT INTO agents (operator_id, name, phone, email, linked_user_id)
SELECT '00000000-0000-4000-8000-000000000001'::uuid, u.name, '', u.email, u.id
FROM "user" u
WHERE u.id IN ('D862T2c4awEabjAJRNY2Cr1DC2MTfyLI', 'EomOhXSQ3HfcIMk5hOMKliraIcEaqQuo', '5bv99BUnYfpzIcbtyC2vcSPzlD0fgV6x')
ON CONFLICT (linked_user_id) DO NOTHING;

-- Five pilgrims: a mahram pair in GROUP-A (SOC-01), a wheelchair pilgrim in
-- GROUP-B (SOC-01), and a mahram pair in GROUP-C (SOC-02) — enough to keep
-- the wheelchair/mahram/kloter-filter demo states without the clutter of a
-- full 20-pilgrim manifest.
INSERT INTO pilgrims (id, season_id, operator_id, group_id, kloter_id, full_name, passport_number, nationality, date_of_birth, gender, phone, emergency_contact, preferred_lang, requires_wheelchair, mahram_id, is_substituted)
VALUES
  ('00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000001001', 'Ahmad Fauzi',  'A1234567', 'ID', '1980-03-12', 'MALE',   '+628111000001', '+628111000001', 'id', false, NULL, false),
  ('00000000-0000-4000-8000-000000000302', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000001001', 'Siti Aminah',  'A1234568', 'ID', '1985-07-22', 'FEMALE', '+628111000002', '+628111000001', 'id', false, '00000000-0000-4000-8000-000000000301', false),
  ('00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000001001', 'Budi Santoso', 'A1234569', 'ID', '1978-11-04', 'MALE',   '+628111000003', '+628111000003', 'id', true,  NULL, false),
  ('00000000-0000-4000-8000-000000000304', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000001002', 'Rina Kartika', 'A1234575', 'ID', '1981-04-11', 'FEMALE', '+628111000009', '+628111000010', 'id', false, '00000000-0000-4000-8000-000000000305', false),
  ('00000000-0000-4000-8000-000000000305', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000001002', 'Hasan Basri',  'A1234576', 'ID', '1976-08-09', 'MALE',   '+628111000010', '+628111000010', 'id', false, '00000000-0000-4000-8000-000000000304', false);

INSERT INTO hotels (id, operator_id, season_id, name, city, star_rating, address, check_in_date, check_out_date)
SELECT '00000000-0000-4000-8000-000000000401'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, 'Hotel Al Safwa Royal Orchid', 'Makkah', 5, 'Ajyad Street, Makkah', start_date::date, (start_date + INTERVAL '10 days')::date FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000000402'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, 'Hotel Movenpick Anabat', 'Madinah', 4, 'Central Area, Madinah', (start_date + INTERVAL '10 days')::date, (start_date + INTERVAL '16 days')::date FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000000403'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, 'Hotel Meridien Jeddah', 'Jeddah', 4, 'Al Hamra District, Jeddah', start_date::date, (start_date + INTERVAL '1 day')::date FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101';

-- Ten rooms per hotel: two single, three double, three triple, and two quad rooms.
INSERT INTO rooms (id, hotel_id, operator_id, room_number, room_type, capacity, floor, gender)
SELECT
  ('00000000-0000-4000-8000-' || lpad((500 + hotel_offset + room_offset)::text, 12, '0'))::uuid,
  hotel_id,
  '00000000-0000-4000-8000-000000000001'::uuid,
  room_number,
  room_type,
  capacity,
  floor,
  gender
FROM (
  SELECT hotel_id, hotel_offset, room_number, room_type, capacity, floor, gender, room_offset
  FROM (VALUES
    ('00000000-0000-4000-8000-000000000401'::uuid, 0),
    ('00000000-0000-4000-8000-000000000402'::uuid, 10),
    ('00000000-0000-4000-8000-000000000403'::uuid, 20)
  ) AS hotels(hotel_id, hotel_offset)
  CROSS JOIN (VALUES
    ('101', 'single', 1, 1, 'male', 1), ('102', 'single', 1, 1, 'female', 2),
    ('103', 'double', 2, 1, 'male', 3), ('104', 'double', 2, 1, 'female', 4), ('105', 'double', 2, 1, 'family', 5),
    ('106', 'triple', 3, 2, 'male', 6), ('107', 'triple', 3, 2, 'female', 7), ('108', 'triple', 3, 2, 'family', 8),
    ('109', 'quad', 4, 2, 'male', 9), ('110', 'quad', 4, 2, 'female', 10)
  ) AS rooms(room_number, room_type, capacity, floor, gender, room_offset)
) AS room_data;

-- All five pilgrims are allocated a room — the mahram pairs each into a
-- 'family' room (105), matching the actual room-gender constraint intent.
INSERT INTO room_allocations (id, room_id, pilgrim_id, operator_id, assigned_by)
VALUES
  ('00000000-0000-4000-8000-000000000601', '00000000-0000-4000-8000-000000000505', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000001', 'system'),
  ('00000000-0000-4000-8000-000000000602', '00000000-0000-4000-8000-000000000505', '00000000-0000-4000-8000-000000000302', '00000000-0000-4000-8000-000000000001', 'system'),
  ('00000000-0000-4000-8000-000000000603', '00000000-0000-4000-8000-000000000501', '00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000001', 'system'),
  ('00000000-0000-4000-8000-000000000604', '00000000-0000-4000-8000-000000000515', '00000000-0000-4000-8000-000000000304', '00000000-0000-4000-8000-000000000001', 'system'),
  ('00000000-0000-4000-8000-000000000605', '00000000-0000-4000-8000-000000000515', '00000000-0000-4000-8000-000000000305', '00000000-0000-4000-8000-000000000001', 'system');

-- All 5 existing movements are tagged to SOC-01 — SOC-02 is left with no
-- movements of its own, demoing the Transportasi kloter filter's empty state.
INSERT INTO movements (id, season_id, operator_id, name, origin, destination, scheduled_at, kloter_id)
SELECT '00000000-0000-4000-8000-000000000701'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, 'Arrival flight CGK to JED', 'CGK', 'JED', start_date + INTERVAL '8 hours', '00000000-0000-4000-8000-000000001001'::uuid FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000000702'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, 'Transfer Jeddah to Makkah', 'JED', 'Makkah', start_date + INTERVAL '14 hours', '00000000-0000-4000-8000-000000001001'::uuid FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000000703'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, 'Transfer Makkah to Madinah', 'Makkah', 'Madinah', start_date + INTERVAL '10 days 9 hours', '00000000-0000-4000-8000-000000001001'::uuid FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000000704'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, 'Transfer Madinah to Jeddah', 'Madinah', 'JED', start_date + INTERVAL '16 days 10 hours', '00000000-0000-4000-8000-000000001001'::uuid FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000000705'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, 'Departure flight JED to CGK', 'JED', 'CGK', start_date + INTERVAL '16 days 18 hours', '00000000-0000-4000-8000-000000001001'::uuid FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101';

INSERT INTO vehicles (id, movement_id, operator_id, plate_number, capacity, driver_name, driver_phone)
VALUES
  ('00000000-0000-4000-8000-000000000801', '00000000-0000-4000-8000-000000000702', '00000000-0000-4000-8000-000000000001', 'B 1234 XYZ', 40, 'Pak Joko', '+628121000001'),
  ('00000000-0000-4000-8000-000000000802', '00000000-0000-4000-8000-000000000702', '00000000-0000-4000-8000-000000000001', 'B 5678 ABC', 40, 'Pak Andi', '+628121000002'),
  ('00000000-0000-4000-8000-000000000803', '00000000-0000-4000-8000-000000000703', '00000000-0000-4000-8000-000000000001', 'B 9012 DEF', 15, 'Pak Rudi', '+628121000003'),
  ('00000000-0000-4000-8000-000000000804', '00000000-0000-4000-8000-000000000704', '00000000-0000-4000-8000-000000000001', 'B 3456 GHI', 15, 'Pak Deni', '+628121000004');

-- A couple of GROUP-A chat messages — pilgrim asks, their Muttawwif (the
-- group's real leader_id) replies — so /leader/[groupId]/chat and the
-- pilgrim PWA's Chat tab both have something to show immediately.
INSERT INTO chat_messages (id, operator_id, group_id, sender_pilgrim_id, body, created_at)
VALUES
  ('00000000-0000-4000-8000-000000000901', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000301', 'Assalamualaikum, jam berapa kumpul di lobby besok?', NOW() - INTERVAL '2 hours');
INSERT INTO chat_messages (id, operator_id, group_id, sender_user_id, body, created_at)
VALUES
  ('00000000-0000-4000-8000-000000000902', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000201', 'D862T2c4awEabjAJRNY2Cr1DC2MTfyLI', 'Wa alaikumsalam, jam 05.00 pagi ya setelah subuh.', NOW() - INTERVAL '1 hour 50 minutes');

-- One Module 7 demo product — margins take the column defaults
-- (15% platform / 70% operator / 15% agent). Lets the pilgrim PWA's
-- Products tab and the checkout flow be tested end-to-end once
-- XENDIT_SECRET_KEY is set; CreateOrder fails fast (no dangling order)
-- until then.
INSERT INTO products (id, operator_id, season_id, name, category, type, price_idr, duration_days, description, is_active)
VALUES ('00000000-0000-4000-8000-000000001101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'eSIM Roaming Saudi 7 Hari', 'ROAMING_DATA', '', 150000, 7, 'Paket data roaming untuk jamaah selama di Arab Saudi', true);

COMMIT;
