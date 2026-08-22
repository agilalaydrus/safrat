-- Local-only Maktour Indonesia (operator "Umroh PUAS") demo fixture.
-- Re-running this script replaces only the deterministic demo operator and its
-- cascaded data (everything hangs off operators.id via ON DELETE CASCADE).
--
-- Better Auth user ids below are hardcoded to THIS local dev database's real
-- signed-up accounts (org "UmrohPUAS", id fb1cIdKNdUdEq4aRzOVLnZZokZ0Dn6qF) —
-- they cannot be invented, Better Auth owns and migrates the "user" table
-- itself. If you reset Better Auth's own tables, re-check these ids still
-- exist before re-running this file:
--   ewOvX4F8eVgunMNu7GBGwryL6bdxcuZA  Agil Al Idrus        owner
--   BoJwKInShRRtlOHKzdul5D7aNH5IaRhx  Admin Demo           admin
--   DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x  Ketua Rombongan A    member (group leader A)
--   8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5  Ketua Rombongan B    member (group leader B)
--   2vbLn59Uu2k1cMC7WnjtUHiPvoTNNJIZ  Tour Leader Demo     member (group leader C / agent portal demo)
--   ZuY9WBDuoJip5Klre3YuI6mS60Oc6RRy  Jamaah Demo          not an org member — pilgrim-linked account
BEGIN;

-- seat_assignments.operator_id lacks ON DELETE CASCADE (unlike every other
-- operator_id FK in the schema) — delete it explicitly first or the cascade
-- delete below fails once any seat_assignments rows exist.
DELETE FROM seat_assignments WHERE operator_id = '00000000-0000-4000-8000-000000000001';
DELETE FROM operators WHERE id = '00000000-0000-4000-8000-000000000001';

INSERT INTO operators (id, better_auth_org_id, name, country, email, license_number, plan, slug)
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
  'PRO',
  'maktour'
;

-- Dates are anchored to NOW() rather than hardcoded — re-running this seed
-- must always produce a current/upcoming season, never one that silently
-- drifts into the past as real time moves on. capacity=8 is deliberately
-- lower than the 10 pilgrims seeded below, so the season is demonstrably
-- "full" and season_waitlists' entries below reflect a real state.
INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, is_active, capacity)
VALUES
  ('00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001',
    'Musim Haji ' || extract(year from NOW() + INTERVAL '5 days')::text, 'HAJJ',
    date_trunc('day', NOW()) + INTERVAL '5 days',
    date_trunc('day', NOW()) + INTERVAL '21 days' + INTERVAL '23 hours 59 minutes 59 seconds',
    true, 8),
  ('00000000-0000-4000-8000-000000000102', '00000000-0000-4000-8000-000000000001',
    'Musim Haji ' || (extract(year from NOW())::int - 1)::text, 'HAJJ',
    date_trunc('day', NOW()) - INTERVAL '410 days',
    date_trunc('day', NOW()) - INTERVAL '390 days',
    false, 0),
  ('00000000-0000-4000-8000-000000000103', '00000000-0000-4000-8000-000000000001',
    'Umrah Ramadhan ' || extract(year from NOW() + INTERVAL '60 days')::text, 'UMRAH_RAMADHAN',
    date_trunc('day', NOW()) + INTERVAL '60 days',
    date_trunc('day', NOW()) + INTERVAL '70 days',
    false, 45);

-- Two kloter (Kemenag departure batches), each anchored to the season's own
-- start_date the same way hotels/movements are — SOC-01 is the earlier
-- batch (departs on the season's actual start date), SOC-02 follows two
-- days later. GROUP-A's pilgrims fly in SOC-01, GROUP-B's in SOC-01, and
-- GROUP-C's in SOC-02.
INSERT INTO kloters (id, operator_id, season_id, code, embarkation, flight_number, departure_date, capacity)
SELECT '00000000-0000-4000-8000-000000001001'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, 'SOC-01', 'Soekarno-Hatta', 'SV 815', start_date, 45 FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000001002'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, 'SOC-02', 'Soekarno-Hatta', 'SV 823', start_date + INTERVAL '2 days', 45 FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101';

-- Three groups, each led by a real Better Auth user id from this local DB
-- (see header) — stable across reseeds, so every group always has a leader
-- on both the admin Groups page and the leader's own /leader app.
INSERT INTO groups (id, season_id, operator_id, name, capacity, leader_id)
VALUES
  ('00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', 'GROUP-A', 40, 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', 'GROUP-B', 40, '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5'),
  ('00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', 'GROUP-C', 40, '2vbLn59Uu2k1cMC7WnjtUHiPvoTNNJIZ');

-- Agents — explicit deterministic ids (not gen_random_uuid()) so every FK
-- below (orders.agent_id, agent_payouts, referral chain) stays stable across
-- reseeds. The 3 group leaders auto-become agents (GroupService.UpdateGroup
-- does this for real assignments made through the app — this seed inserts
-- groups directly, bypassing that service, so it's done explicitly here).
-- A 4th agent (independent, not a group leader) demonstrates the plain
-- agent-management + referral-tier flow on its own.
INSERT INTO agents (id, operator_id, name, phone, email, linked_user_id, tier, referral_code, commission_rate, is_active)
VALUES
  ('00000000-0000-4000-8000-000000001201', '00000000-0000-4000-8000-000000000001', 'Ketua Rombongan A', '+628131000001', 'leader1.demo@safrat.test', 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x', 'GOLD', 'AGTGOLD1', 0.10, true),
  ('00000000-0000-4000-8000-000000001202', '00000000-0000-4000-8000-000000000001', 'Ketua Rombongan B', '+628131000002', 'leader2.demo@safrat.test', '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5', 'SILVER', 'AGTSILV2', 0.08, true),
  ('00000000-0000-4000-8000-000000001203', '00000000-0000-4000-8000-000000000001', 'Tour Leader Demo', '+628131000003', 'tourleader.demo@safrat.test', '2vbLn59Uu2k1cMC7WnjtUHiPvoTNNJIZ', 'SILVER', 'AGTSILV3', 0.08, true),
  ('00000000-0000-4000-8000-000000001204', '00000000-0000-4000-8000-000000000001', 'Mitra Rekanan Jaya', '+628131000004', 'mitra.jaya@example.test', NULL, 'BRONZE', 'AGTBRNZ4', 0.05, true);

-- Referral chain: agent 1204 (Mitra Rekanan Jaya, BRONZE) was referred by
-- agent 1201 (Ketua Rombongan A, GOLD) — exercises agents.referred_by_agent_id.
UPDATE agents SET referred_by_agent_id = '00000000-0000-4000-8000-000000001201'
WHERE id = '00000000-0000-4000-8000-000000001204';

-- Ten pilgrims: two mahram pairs (301/302 in GROUP-A/SOC-01, 304/305 in
-- GROUP-C/SOC-02), one wheelchair pilgrim (303), one substituted pilgrim
-- (306, replaced by 307 — is_substituted / substituted_by_id / reason /
-- substituted_at all set, matching the "irreversible + must write an audit
-- log" rule below), one cancelled pilgrim (308, status='CANCELLED', matched
-- by a pilgrim_cancellations row), one pilgrim linked to a real Better Auth
-- account via email match (309 -> Jamaah Demo), and one pilgrim with a full
-- profile (310: insurance, agent-referred registration, complete documents).
INSERT INTO pilgrims (
  id, season_id, operator_id, group_id, kloter_id, agent_id, full_name, passport_number, nationality,
  date_of_birth, gender, phone, emergency_contact, emergency_contact_name, emergency_contact_phone,
  preferred_lang, requires_wheelchair, mahram_id, is_substituted, substituted_by_id, substitution_reason, substituted_at,
  email, linked_user_id, payment_status, hotel_checked_in, documents_passport, documents_photo, documents_vaccine,
  passport_expiry_date, vaccine_meningitis_date, status,
  insurance_provider, insurance_policy_no, insurance_class, blood_type
)
VALUES
  ('00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000001001', NULL, 'Ahmad Fauzi',  'A1234567', 'ID', '1980-03-12', 'MALE',   '+628111000001', '+628111000001', 'Siti Aminah', '+628111000002', 'id', false, NULL, false, NULL, '', NULL, NULL, NULL, 'PAID', true, true, true, true, CURRENT_DATE + INTERVAL '2 years', CURRENT_DATE - INTERVAL '1 year', 'ACTIVE', 'Askrindo', 'ASK-0001', 'PLUS', 'O+'),
  ('00000000-0000-4000-8000-000000000302', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000001001', NULL, 'Siti Aminah',  'A1234568', 'ID', '1985-07-22', 'FEMALE', '+628111000002', '+628111000001', 'Ahmad Fauzi', '+628111000001', 'id', false, '00000000-0000-4000-8000-000000000301', false, NULL, '', NULL, NULL, NULL, 'PAID', true, true, true, true, CURRENT_DATE + INTERVAL '2 years', CURRENT_DATE - INTERVAL '1 year', 'ACTIVE', 'Askrindo', 'ASK-0002', 'PLUS', 'A+'),
  ('00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000001001', NULL, 'Budi Santoso', 'A1234569', 'ID', '1978-11-04', 'MALE',   '+628111000003', '+628111000003', 'Wati Santoso', '+628111000004', 'id', true,  NULL, false, NULL, '', NULL, NULL, NULL, 'DP',   false, true, true, false, CURRENT_DATE + INTERVAL '2 years', CURRENT_DATE - INTERVAL '1 year', 'ACTIVE', 'Askrindo', 'ASK-0003', 'STANDARD', 'B+'),
  ('00000000-0000-4000-8000-000000000304', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000001002', NULL, 'Rina Kartika', 'A1234575', 'ID', '1981-04-11', 'FEMALE', '+628111000009', '+628111000010', 'Hasan Basri', '+628111000010', 'id', false, '00000000-0000-4000-8000-000000000305', false, NULL, '', NULL, NULL, NULL, 'PAID', false, true, true, true, CURRENT_DATE + INTERVAL '3 years', CURRENT_DATE - INTERVAL '2 months', 'ACTIVE', 'Jasindo', 'JAS-0009', 'STANDARD', 'AB+'),
  ('00000000-0000-4000-8000-000000000305', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000001002', NULL, 'Hasan Basri',  'A1234576', 'ID', '1976-08-09', 'MALE',   '+628111000010', '+628111000010', 'Rina Kartika', '+628111000009', 'id', false, NULL, false, NULL, '', NULL, NULL, NULL, 'PAID', false, true, true, true, CURRENT_DATE + INTERVAL '3 years', CURRENT_DATE - INTERVAL '2 months', 'ACTIVE', 'Jasindo', 'JAS-0010', 'STANDARD', 'O-'),
  ('00000000-0000-4000-8000-000000000306', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000001001', NULL, 'Dewi Lestari', 'A1234580', 'ID', '1990-02-14', 'FEMALE', '+628111000011', '+628111000011', 'Rudi Lestari', '+628111000012', 'id', false, NULL, true, '00000000-0000-4000-8000-000000000307', 'Sakit keras, tidak bisa berangkat — digantikan oleh Fitri Handayani (adik kandung).', NOW() - INTERVAL '3 days', NULL, NULL, 'PAID', false, true, true, true, CURRENT_DATE + INTERVAL '2 years', CURRENT_DATE - INTERVAL '1 year', 'ACTIVE', 'Askrindo', 'ASK-0011', 'STANDARD', 'B-'),
  ('00000000-0000-4000-8000-000000000307', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000001001', NULL, 'Fitri Handayani', 'A1234581', 'ID', '1992-06-30', 'FEMALE', '+628111000013', '+628111000013', 'Rudi Lestari', '+628111000012', 'id', false, NULL, false, NULL, '', NULL, NULL, NULL, 'PAID', false, true, true, true, CURRENT_DATE + INTERVAL '2 years', CURRENT_DATE - INTERVAL '1 year', 'ACTIVE', 'Askrindo', 'ASK-0012', 'STANDARD', 'B-'),
  ('00000000-0000-4000-8000-000000000308', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000001001', NULL, 'Joko Prasetyo', 'A1234582', 'ID', '1975-01-20', 'MALE',   '+628111000014', '+628111000014', 'Ani Prasetyo', '+628111000015', 'id', false, NULL, false, NULL, '', NULL, NULL, NULL, 'DP',   false, true, false, false, CURRENT_DATE + INTERVAL '1 years', NULL, 'CANCELLED', '', '', 'STANDARD', ''),
  ('00000000-0000-4000-8000-000000000309', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000001001', NULL, 'Maryam Ulfa', 'A1234583', 'ID', '1988-09-17', 'FEMALE', '+628111000016', '+628111000016', 'Zainal Ulfa', '+628111000017', 'id', false, NULL, false, NULL, '', NULL, 'pilgrim.demo@safrat.test', 'ZuY9WBDuoJip5Klre3YuI6mS60Oc6RRy', 'PAID', false, true, true, true, CURRENT_DATE + INTERVAL '2 years', CURRENT_DATE - INTERVAL '1 year', 'ACTIVE', 'Jasindo', 'JAS-0016', 'PREMIUM', 'A-'),
  ('00000000-0000-4000-8000-000000000310', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000203', '00000000-0000-4000-8000-000000001002', '00000000-0000-4000-8000-000000001204', 'Agus Salim', 'A1234584', 'ID', '1983-12-05', 'MALE', '+628111000018', '+628111000018', 'Nur Salim', '+628111000019', 'id', false, NULL, false, NULL, '', NULL, NULL, NULL, 'PAID', false, true, true, true, CURRENT_DATE + INTERVAL '4 years', CURRENT_DATE - INTERVAL '3 months', 'ACTIVE', 'Jasindo', 'JAS-0018', 'PREMIUM', 'O+');

-- Last-known GPS ping (pilgrims.last_lat/lng/last_location_at) for a couple
-- of pilgrims — independent of any SOS event, matching the periodic-ping
-- design (see migration 031).
UPDATE pilgrims SET last_lat = 21.4225, last_lng = 39.8262, last_location_at = NOW() - INTERVAL '4 minutes' WHERE id = '00000000-0000-4000-8000-000000000303';
UPDATE pilgrims SET last_lat = 21.3891, last_lng = 39.8579, last_location_at = NOW() - INTERVAL '9 minutes' WHERE id = '00000000-0000-4000-8000-000000000309';

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

-- Room allocations for the 9 active (non-cancelled) pilgrims — the mahram
-- pairs each into a 'family' room, 306/307 (unrelated, same gender) share a
-- female double, 303/309/310 get single-gender rooms. 308 (CANCELLED) is
-- deliberately left unallocated — a cancelled pilgrim holds no room.
INSERT INTO room_allocations (id, room_id, pilgrim_id, operator_id, assigned_by)
VALUES
  ('00000000-0000-4000-8000-000000000601', '00000000-0000-4000-8000-000000000505', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000001', 'system'),
  ('00000000-0000-4000-8000-000000000602', '00000000-0000-4000-8000-000000000505', '00000000-0000-4000-8000-000000000302', '00000000-0000-4000-8000-000000000001', 'system'),
  ('00000000-0000-4000-8000-000000000603', '00000000-0000-4000-8000-000000000501', '00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000001', 'system'),
  ('00000000-0000-4000-8000-000000000604', '00000000-0000-4000-8000-000000000515', '00000000-0000-4000-8000-000000000304', '00000000-0000-4000-8000-000000000001', 'system'),
  ('00000000-0000-4000-8000-000000000605', '00000000-0000-4000-8000-000000000515', '00000000-0000-4000-8000-000000000305', '00000000-0000-4000-8000-000000000001', 'system'),
  ('00000000-0000-4000-8000-000000000606', '00000000-0000-4000-8000-000000000504', '00000000-0000-4000-8000-000000000306', '00000000-0000-4000-8000-000000000001', 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000000607', '00000000-0000-4000-8000-000000000504', '00000000-0000-4000-8000-000000000307', '00000000-0000-4000-8000-000000000001', 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000000608', '00000000-0000-4000-8000-000000000507', '00000000-0000-4000-8000-000000000309', '00000000-0000-4000-8000-000000000001', '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5'),
  ('00000000-0000-4000-8000-000000000609', '00000000-0000-4000-8000-000000000516', '00000000-0000-4000-8000-000000000310', '00000000-0000-4000-8000-000000000001', '2vbLn59Uu2k1cMC7WnjtUHiPvoTNNJIZ');

-- All 5 movements are tagged to SOC-01 — SOC-02 is left with no movements of
-- its own, demoing the Transportasi kloter filter's empty state (304/305/310
-- ride nothing, on purpose).
INSERT INTO movements (id, season_id, operator_id, name, origin, destination, scheduled_at, kloter_id, mode)
SELECT '00000000-0000-4000-8000-000000000701'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, 'Arrival flight CGK to JED', 'CGK', 'JED', start_date + INTERVAL '8 hours', '00000000-0000-4000-8000-000000001001'::uuid, 'FLIGHT' FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000000702'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, 'Transfer Jeddah to Makkah', 'JED', 'Makkah', start_date + INTERVAL '14 hours', '00000000-0000-4000-8000-000000001001'::uuid, 'BUS' FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000000703'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, 'Transfer Makkah to Madinah', 'Makkah', 'Madinah', start_date + INTERVAL '10 days 9 hours', '00000000-0000-4000-8000-000000001001'::uuid, 'TRAIN' FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000000704'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, 'Transfer Madinah to Jeddah', 'Madinah', 'JED', start_date + INTERVAL '16 days 10 hours', '00000000-0000-4000-8000-000000001001'::uuid, 'BUS' FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101'
UNION ALL
SELECT '00000000-0000-4000-8000-000000000705'::uuid, '00000000-0000-4000-8000-000000000101'::uuid, '00000000-0000-4000-8000-000000000001'::uuid, 'Departure flight JED to CGK', 'JED', 'CGK', start_date + INTERVAL '16 days 18 hours', '00000000-0000-4000-8000-000000001001'::uuid, 'FLIGHT' FROM seasons WHERE id = '00000000-0000-4000-8000-000000000101';

INSERT INTO vehicles (id, movement_id, operator_id, plate_number, capacity, driver_name, driver_phone)
VALUES
  ('00000000-0000-4000-8000-000000000801', '00000000-0000-4000-8000-000000000702', '00000000-0000-4000-8000-000000000001', 'B 1234 XYZ', 40, 'Pak Joko', '+628121000001'),
  ('00000000-0000-4000-8000-000000000802', '00000000-0000-4000-8000-000000000702', '00000000-0000-4000-8000-000000000001', 'B 5678 ABC', 40, 'Pak Andi', '+628121000002'),
  ('00000000-0000-4000-8000-000000000803', '00000000-0000-4000-8000-000000000703', '00000000-0000-4000-8000-000000000001', 'B 9012 DEF', 15, 'Pak Rudi', '+628121000003'),
  ('00000000-0000-4000-8000-000000000804', '00000000-0000-4000-8000-000000000704', '00000000-0000-4000-8000-000000000001', 'B 3456 GHI', 15, 'Pak Deni', '+628121000004');

-- Seat assignments — every SOC-01 pilgrim who rides a ground movement (all
-- except the cancelled 308 and the SOC-02 pilgrims, who have no movements at
-- all) gets a seat, and the two mahram pairs riding together (301/302) share
-- the SAME vehicle on every leg, per the "mahram pairs must share a vehicle"
-- rule.
INSERT INTO seat_assignments (id, vehicle_id, pilgrim_id, operator_id, seat_number, assigned_by)
VALUES
  ('00000000-0000-4000-8000-000000000811', '00000000-0000-4000-8000-000000000801', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000001', 1, 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000000812', '00000000-0000-4000-8000-000000000801', '00000000-0000-4000-8000-000000000302', '00000000-0000-4000-8000-000000000001', 2, 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000000813', '00000000-0000-4000-8000-000000000801', '00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000001', 3, '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5'),
  ('00000000-0000-4000-8000-000000000814', '00000000-0000-4000-8000-000000000802', '00000000-0000-4000-8000-000000000306', '00000000-0000-4000-8000-000000000001', 1, 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000000815', '00000000-0000-4000-8000-000000000802', '00000000-0000-4000-8000-000000000307', '00000000-0000-4000-8000-000000000001', 2, 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000000816', '00000000-0000-4000-8000-000000000802', '00000000-0000-4000-8000-000000000309', '00000000-0000-4000-8000-000000000001', 3, '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5'),
  ('00000000-0000-4000-8000-000000000821', '00000000-0000-4000-8000-000000000803', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000001', 1, 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000000822', '00000000-0000-4000-8000-000000000803', '00000000-0000-4000-8000-000000000302', '00000000-0000-4000-8000-000000000001', 2, 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000000823', '00000000-0000-4000-8000-000000000804', '00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000001', 1, '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5');

-- Check-ins: departure at CGK (movement 701, type ARRIVAL means "arrived at
-- destination" per the CHECK constraint's two values — used here for the
-- inbound flight) and boarding confirmation for the outbound leg (movement
-- 702, DEPARTURE), for a few SOC-01 pilgrims.
INSERT INTO check_ins (id, operator_id, movement_id, pilgrim_id, type, checked_in_by)
VALUES
  ('00000000-0000-4000-8000-000000001501', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000701', '00000000-0000-4000-8000-000000000301', 'ARRIVAL',   'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000001502', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000701', '00000000-0000-4000-8000-000000000302', 'ARRIVAL',   'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000001503', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000702', '00000000-0000-4000-8000-000000000301', 'DEPARTURE', 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x'),
  ('00000000-0000-4000-8000-000000001504', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000702', '00000000-0000-4000-8000-000000000303', 'DEPARTURE', '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5');

-- SOS alerts across every status: ACTIVE (fresh, no responder yet, current
-- GPS snapshot), ACKNOWLEDGED (a leader has seen it), ESCALATED (past the
-- worker's 10-minute window, would have pushed coordinators), RESOLVED
-- (fully closed out).
INSERT INTO sos_alerts (id, operator_id, pilgrim_id, status, acknowledged_by, acknowledged_at, resolved_by, resolved_at, notes, lat, lng, created_at)
VALUES
  ('00000000-0000-4000-8000-000000001601', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000303', 'ACTIVE',       NULL,                                NULL,                       NULL,                                NULL,                       '', 21.4225, 39.8262, NOW() - INTERVAL '2 minutes'),
  ('00000000-0000-4000-8000-000000001602', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000306', 'ACKNOWLEDGED', 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x', NOW() - INTERVAL '20 minutes', NULL,                                NULL,                       'Sedang menuju lokasi.', 21.4235, 39.8241, NOW() - INTERVAL '25 minutes'),
  ('00000000-0000-4000-8000-000000001603', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000309', 'ESCALATED',    NULL,                                NULL,                       NULL,                                NULL,                       'Belum ada respon leader dalam 10 menit.', 21.3891, 39.8579, NOW() - INTERVAL '40 minutes'),
  ('00000000-0000-4000-8000-000000001604', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000301', 'RESOLVED',     '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5', NOW() - INTERVAL '2 days 3 hours', 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', NOW() - INTERVAL '2 days 2 hours', 'Jamaah sudah ditemukan, hanya tersesat sebentar.', 21.4188, 39.8262, NOW() - INTERVAL '2 days 4 hours');

-- Push subscriptions for the admin + both leaders (dummy FCM tokens — real
-- ones only ever come from a real browser registration).
INSERT INTO push_subscriptions (id, operator_id, user_id, fcm_token)
VALUES
  ('00000000-0000-4000-8000-000000001701', '00000000-0000-4000-8000-000000000001', 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', 'demo-fcm-token-admin-0001'),
  ('00000000-0000-4000-8000-000000001702', '00000000-0000-4000-8000-000000000001', 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x', 'demo-fcm-token-leader1-0001'),
  ('00000000-0000-4000-8000-000000001703', '00000000-0000-4000-8000-000000000001', '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5', 'demo-fcm-token-leader2-0001');

-- A couple of GROUP-A chat messages — pilgrim asks, their Muttawwif (the
-- group's real leader_id) replies — so /leader/[groupId]/chat and the
-- pilgrim PWA's Chat tab both have something to show immediately.
INSERT INTO chat_messages (id, operator_id, group_id, sender_pilgrim_id, body, created_at)
VALUES
  ('00000000-0000-4000-8000-000000000901', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000301', 'Assalamualaikum, jam berapa kumpul di lobby besok?', NOW() - INTERVAL '2 hours');
INSERT INTO chat_messages (id, operator_id, group_id, sender_user_id, body, created_at)
VALUES
  ('00000000-0000-4000-8000-000000000902', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000201', 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x', 'Wa alaikumsalam, jam 05.00 pagi ya setelah subuh.', NOW() - INTERVAL '1 hour 50 minutes');

-- Broadcasts — season-wide announcements from the operator.
INSERT INTO broadcasts (id, operator_id, season_id, title, body, created_at)
VALUES
  ('00000000-0000-4000-8000-000000001801', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Jadwal Manasik Terakhir', 'Manasik terakhir akan dilaksanakan H-3 sebelum keberangkatan di Aula Kantor Pusat.', NOW() - INTERVAL '1 day'),
  ('00000000-0000-4000-8000-000000001802', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Cuaca Panas di Makkah', 'Suhu di Makkah diperkirakan mencapai 42°C, jamaah diimbau membawa payung dan air minum.', NOW() - INTERVAL '6 hours');

-- Four Module 7 demo products spanning every category, margins always
-- summing to <= 1.0 (CODEX_SPEC.md §7).
INSERT INTO products (id, operator_id, season_id, name, category, type, price_idr, duration_days, description, is_active, platform_margin_pct, operator_margin_pct, agent_margin_pct)
VALUES
  ('00000000-0000-4000-8000-000000001101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'eSIM Roaming Saudi 7 Hari', 'ROAMING_DATA', '', 150000, 7, 'Paket data roaming untuk jamaah selama di Arab Saudi', true, 0.15, 0.70, 0.15),
  ('00000000-0000-4000-8000-000000001102', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Paket Haji Reguler 21 Hari', 'TRAVEL_PACKAGE', 'HAJJ', 65000000, 21, 'Paket perjalanan Haji reguler termasuk hotel bintang 4-5, transportasi, dan katering', true, 0.05, 0.85, 0.10),
  ('00000000-0000-4000-8000-000000001103', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Koper Kabin + Perlengkapan Ihram', 'EQUIPMENT', '', 750000, 0, 'Set koper kabin, mukena/ihram, dan perlengkapan ibadah', true, 0.15, 0.70, 0.15),
  ('00000000-0000-4000-8000-000000001104', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Pulsa & Top-up PPOB 100rb', 'PPOB_CREDIT', '', 100000, 0, 'Top-up pulsa dan pembayaran PPOB untuk jamaah', true, 0.10, 0.80, 0.10);

-- Orders: PAID with agent commission (301, agent 1201 = product 1102's
-- 0.10 agent margin on 65,000,000 = 6,500,000), PAID with NO agent (303 —
-- agent_commission_idr is 0 and operator_amount_idr absorbs what would have
-- been the agent's cut, per "agentCommission = 0 when there's no agentId"),
-- PENDING with a live Xendit invoice (309), EXPIRED (304, agent 1202), and
-- CANCELLED (308 — the cancelled pilgrim's abandoned order).
INSERT INTO orders (id, operator_id, season_id, pilgrim_id, product_id, agent_id, quantity, unit_price_idr, total_price_idr, platform_amount_idr, operator_amount_idr, agent_commission_idr, status, xendit_invoice_id, xendit_invoice_url, paid_at, created_at)
VALUES
  ('00000000-0000-4000-8000-000000001301', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000001102', '00000000-0000-4000-8000-000000001201', 1, 65000000, 65000000, 3250000, 55250000, 6500000, 'PAID',    'demo-inv-0001', 'https://checkout.xendit.co/demo-inv-0001', NOW() - INTERVAL '5 days', NOW() - INTERVAL '6 days'),
  ('00000000-0000-4000-8000-000000001302', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000001101', NULL,                                       1, 150000,   150000,   22500,   127500,   0,       'PAID',    'demo-inv-0002', 'https://checkout.xendit.co/demo-inv-0002', NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days'),
  ('00000000-0000-4000-8000-000000001303', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000309', '00000000-0000-4000-8000-000000001103', '00000000-0000-4000-8000-000000001202', 1, 750000,   750000,   112500,  525000,   112500,  'PENDING', 'demo-inv-0003', 'https://checkout.xendit.co/demo-inv-0003', NULL,                       NOW() - INTERVAL '2 hours'),
  ('00000000-0000-4000-8000-000000001304', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000304', '00000000-0000-4000-8000-000000001104', '00000000-0000-4000-8000-000000001202', 1, 100000,   100000,   10000,   80000,    10000,   'EXPIRED', 'demo-inv-0004', 'https://checkout.xendit.co/demo-inv-0004', NULL,                       NOW() - INTERVAL '10 days'),
  ('00000000-0000-4000-8000-000000001305', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000308', '00000000-0000-4000-8000-000000001101', NULL,                                       1, 150000,   150000,   22500,   127500,   0,       'CANCELLED', NULL, NULL, NULL, NOW() - INTERVAL '15 days');

-- Agent payouts ledger: agent 1201 earned 6,500,000 from order 1301 (PAID).
-- A payout REQUEST was raised by the leader, then APPROVED and turned into a
-- real ledger row (partial payout — 4,000,000 of the 6,500,000 owed, leaving
-- an outstanding balance the app derives from orders minus payouts). A
-- second request from agent 1202 is still PENDING (not yet resolved).
INSERT INTO agent_payout_requests (id, operator_id, agent_id, amount_idr, note, status, resolution_note, resolved_at, resolved_by_user_id, requested_at)
VALUES
  ('00000000-0000-4000-8000-000000001901', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000001201', 4000000, 'Penarikan komisi order Haji Reguler', 'APPROVED', 'Disetujui, transfer BCA.', NOW() - INTERVAL '1 day', 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', NOW() - INTERVAL '2 days'),
  ('00000000-0000-4000-8000-000000001902', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000001202', 112500, 'Penarikan komisi order PPOB', 'PENDING', '', NULL, NULL, NOW() - INTERVAL '3 hours');

INSERT INTO agent_payouts (id, operator_id, agent_id, amount_idr, note, paid_by_user_id, method, request_id, created_at)
VALUES
  ('00000000-0000-4000-8000-000000001202', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000001201', 4000000, 'Penarikan komisi order Haji Reguler', 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', 'TRANSFER', '00000000-0000-4000-8000-000000001901', NOW() - INTERVAL '1 day');

-- Audit logs — the substitution above (306 -> 307) MUST have exactly one
-- matching entry (irreversible action rule). A couple of other real actions
-- are logged too (room reassignment, cancellation) so the Documents/Audit
-- surfaces aren't empty.
INSERT INTO audit_logs (id, operator_id, user_id, action, entity_type, entity_id, metadata, created_at)
VALUES
  ('00000000-0000-4000-8000-000000001401', '00000000-0000-4000-8000-000000000001', 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x', 'SUBSTITUTE', 'pilgrim', '00000000-0000-4000-8000-000000000306', jsonb_build_object('substitutedById', '00000000-0000-4000-8000-000000000307', 'reason', 'Sakit keras, tidak bisa berangkat — digantikan oleh Fitri Handayani (adik kandung).'), NOW() - INTERVAL '3 days'),
  ('00000000-0000-4000-8000-000000001402', '00000000-0000-4000-8000-000000000001', 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', 'CANCEL', 'pilgrim', '00000000-0000-4000-8000-000000000308', jsonb_build_object('reason', 'Kendala biaya mendadak.'), NOW() - INTERVAL '15 days'),
  ('00000000-0000-4000-8000-000000001403', '00000000-0000-4000-8000-000000000001', '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5', 'ALLOCATE_ROOM', 'pilgrim', '00000000-0000-4000-8000-000000000309', jsonb_build_object('roomId', '00000000-0000-4000-8000-000000000507'), NOW() - INTERVAL '6 days');

-- Pilgrim registrations (public /register/[operatorId]/[seasonId] intake) —
-- PENDING (fresh, unreviewed), APPROVED (agent-referred, via 057's agent_id
-- column), and REJECTED.
INSERT INTO pilgrim_registrations (id, operator_id, season_id, product_id, agent_id, full_name, passport_number, date_of_birth, gender, phone, email, nationality, address, status, notes, created_at)
VALUES
  ('00000000-0000-4000-8000-000000001901', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000001102', NULL,                                       'Yusuf Ibrahim', 'A1234590', '1982-05-01', 'MALE',   '+628111000020', 'yusuf.ibrahim@example.test', 'IDN', 'Jl. Melati No. 10, Jakarta', 'PENDING',  '', NOW() - INTERVAL '1 hour'),
  ('00000000-0000-4000-8000-000000001902', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000001102', '00000000-0000-4000-8000-000000001204', 'Halimah Zahra', 'A1234591', '1990-08-19', 'FEMALE', '+628111000021', 'halimah.zahra@example.test', 'IDN', 'Jl. Kenanga No. 5, Bandung', 'APPROVED', 'Sudah lengkap dokumen.', NOW() - INTERVAL '3 days'),
  ('00000000-0000-4000-8000-000000001903', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', NULL,                                       NULL,                                       'Bambang Wijaya', 'A1234592', '1970-01-30', 'MALE', '+628111000022', 'bambang.wijaya@example.test', 'IDN', 'Jl. Anggrek No. 8, Surabaya', 'REJECTED', 'Paspor kurang dari 6 bulan masa berlaku.', NOW() - INTERVAL '5 days');

-- Pilgrim documents — passport, photo, vaccine certificate for a couple of pilgrims.
INSERT INTO pilgrim_documents (id, pilgrim_id, operator_id, doc_type, file_url, file_name, uploaded_by, created_at)
VALUES
  ('00000000-0000-4000-8000-000000001A01', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000001', 'PASSPORT', 'https://storage.example.test/docs/passport-301.pdf', 'passport_ahmad_fauzi.pdf', 'operator', NOW() - INTERVAL '20 days'),
  ('00000000-0000-4000-8000-000000001A02', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000001', 'VACCINE',  'https://storage.example.test/docs/vaccine-301.pdf',  'vaksin_meningitis_ahmad_fauzi.pdf', 'operator', NOW() - INTERVAL '20 days'),
  ('00000000-0000-4000-8000-000000001A03', '00000000-0000-4000-8000-000000000309', '00000000-0000-4000-8000-000000000001', 'PHOTO',    'https://storage.example.test/docs/photo-309.jpg',    'foto_maryam_ulfa.jpg', 'pilgrim', NOW() - INTERVAL '2 days');

-- Season waitlist (season capacity=8, we have 9 active pilgrims + these 3
-- waitlisted — a realistic "full season" state): WAITING (next in line),
-- PROMOTED (a slot opened up, awaiting confirmation), EXPIRED (didn't
-- confirm in time).
INSERT INTO season_waitlists (id, operator_id, season_id, full_name, email, phone, product_id, position, status, promoted_at, expires_at, created_at)
VALUES
  ('00000000-0000-4000-8000-000000001B01', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Wahyu Nugroho', 'wahyu.nugroho@example.test', '+628111000023', '00000000-0000-4000-8000-000000001102', 1, 'WAITING',  NULL,                     NULL,                     NOW() - INTERVAL '2 days'),
  ('00000000-0000-4000-8000-000000001B02', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Sri Mulyani',   'sri.mulyani@example.test',   '+628111000024', '00000000-0000-4000-8000-000000001102', 2, 'PROMOTED', NOW() - INTERVAL '6 hours', NOW() + INTERVAL '18 hours', NOW() - INTERVAL '4 days'),
  ('00000000-0000-4000-8000-000000001B03', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Herman Susanto', 'herman.susanto@example.test', '+628111000025', NULL,                                       3, 'EXPIRED',  NOW() - INTERVAL '10 days', NOW() - INTERVAL '8 days',  NOW() - INTERVAL '12 days');

-- Cancellation policy tiers (sort_order ASC — first tier where min_days <=
-- days_before_departure wins) and the one immutable cancellation record for
-- pilgrim 308.
INSERT INTO cancellation_policies (id, operator_id, season_id, name, min_days, refund_pct, sort_order)
VALUES
  ('00000000-0000-4000-8000-000000001C01', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Pembatalan >= 90 hari', 90, 100, 1),
  ('00000000-0000-4000-8000-000000001C02', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Pembatalan >= 60 hari', 60, 75,  2),
  ('00000000-0000-4000-8000-000000001C03', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Pembatalan >= 30 hari', 30, 50,  3),
  ('00000000-0000-4000-8000-000000001C04', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Pembatalan < 30 hari',   0,  0,  4);

INSERT INTO pilgrim_cancellations (id, pilgrim_id, operator_id, season_id, reason, days_before, refund_pct, refund_amount_idr, total_paid_idr, cancelled_by, cancelled_at, policy_id)
VALUES
  ('00000000-0000-4000-8000-000000001D01', '00000000-0000-4000-8000-000000000308', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Kendala biaya mendadak.', 12, 0, 0, 0, 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', NOW() - INTERVAL '15 days', '00000000-0000-4000-8000-000000001C04');

-- Vendor payments (season-level obligations) across every status.
INSERT INTO vendor_payments (id, operator_id, season_id, vendor_name, category, description, amount_idr, due_date, status, paid_at, created_at)
VALUES
  ('00000000-0000-4000-8000-000000001E01', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Hotel Al Safwa Royal Orchid', 'HOTEL',     'DP 30% blok kamar Makkah', 180000000, (NOW() + INTERVAL '3 days')::date,  'PENDING', NULL,                     NOW() - INTERVAL '10 days'),
  ('00000000-0000-4000-8000-000000001E02', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'PT Bus Al Haramain',          'TRANSPORT', 'Sewa bus AC 4 unit',       45000000,  (NOW() - INTERVAL '2 days')::date,  'PAID',    NOW() - INTERVAL '3 days', NOW() - INTERVAL '20 days'),
  ('00000000-0000-4000-8000-000000001E03', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Katering Zamzam Berkah',      'CATERING',  'Katering harian jamaah',   60000000,  (NOW() - INTERVAL '5 days')::date,  'OVERDUE', NULL,                     NOW() - INTERVAL '25 days'),
  ('00000000-0000-4000-8000-000000001E04', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Asuransi Jasindo',            'INSURANCE', 'Premi asuransi jamaah',    15000000,  (NOW() + INTERVAL '10 days')::date, 'PENDING', NULL,                     NOW() - INTERVAL '2 days');

-- Vendor contracts (SLA tracking) with immutable event logs.
INSERT INTO vendor_contracts (id, operator_id, season_id, vendor_name, vendor_type, contract_number, committed_units, confirmed_units, confirmation_deadline, rate_per_unit_idr, deposit_amount_idr, deposit_paid, status, notes, contact_name, contact_phone)
VALUES
  ('00000000-0000-4000-8000-000000001F01', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Hotel Al Safwa Royal Orchid', 'HOTEL',     'HTL-2026-001', 30, 25, (NOW() + INTERVAL '5 days')::date, 20000000, 180000000, true,  'PARTIAL',   'Menunggu konfirmasi 5 unit sisa.', 'Abdullah Rashid', '+966501000001'),
  ('00000000-0000-4000-8000-000000001F02', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'PT Bus Al Haramain',          'TRANSPORT', 'TRP-2026-002', 4,  4,  (NOW() - INTERVAL '10 days')::date, 11250000, 45000000,  true,  'CONFIRMED', 'Kontrak final, semua unit dikonfirmasi.', 'Pak Jamil', '+628121000099');

INSERT INTO vendor_contract_events (id, contract_id, operator_id, event_type, description, recorded_by, created_at)
VALUES
  ('00000000-0000-4000-8000-000000002001', '00000000-0000-4000-8000-000000001F01', '00000000-0000-4000-8000-000000000001', 'DEPOSIT_PAID', 'DP 30% telah ditransfer ke hotel.', 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', NOW() - INTERVAL '18 days'),
  ('00000000-0000-4000-8000-000000002002', '00000000-0000-4000-8000-000000001F01', '00000000-0000-4000-8000-000000000001', 'PARTIAL_CONFIRM', '25 dari 30 kamar sudah dikonfirmasi pihak hotel.', 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', NOW() - INTERVAL '4 days'),
  ('00000000-0000-4000-8000-000000002003', '00000000-0000-4000-8000-000000001F02', '00000000-0000-4000-8000-000000000001', 'CONFIRMED', 'Seluruh 4 unit bus dikonfirmasi dan kontrak difinalisasi.', 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', NOW() - INTERVAL '15 days');

-- Kloter staff — coordinator/medical/guide roles assigned across both kloters.
INSERT INTO kloter_staff (id, operator_id, kloter_id, staff_id, staff_name, staff_email, role, duties)
VALUES
  ('00000000-0000-4000-8000-000000002101', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000001001', 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', 'Admin Demo', 'admin.demo@safrat.test', 'COORDINATOR', 'Koordinasi umum kloter SOC-01'),
  ('00000000-0000-4000-8000-000000002102', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000001001', 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x', 'Ketua Rombongan A', 'leader1.demo@safrat.test', 'GUIDE', 'Pemandu ibadah Rombongan A'),
  ('00000000-0000-4000-8000-000000002103', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000001002', '2vbLn59Uu2k1cMC7WnjtUHiPvoTNNJIZ', 'Tour Leader Demo', 'tourleader.demo@safrat.test', 'MEDICAL', 'Pendamping medis kloter SOC-02');

-- Insurance claims — one FILED (fresh), one SETTLED (fully paid out).
INSERT INTO insurance_claims (id, pilgrim_id, operator_id, claim_type, incident_date, description, status, claim_amount_idr, settled_amount_idr, filed_by, created_at)
VALUES
  ('00000000-0000-4000-8000-000000002201', '00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000001', 'MEDICAL', (NOW() - INTERVAL '2 days')::date, 'Jamaah mengalami dehidrasi berat, dirawat di klinik Makkah.', 'FILED', 3500000, NULL, 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x', NOW() - INTERVAL '2 days'),
  ('00000000-0000-4000-8000-000000002202', '00000000-0000-4000-8000-000000000305', '00000000-0000-4000-8000-000000000001', 'BAGGAGE', (NOW() - INTERVAL '18 days')::date, 'Koper hilang saat transit di Jeddah, ditemukan dan diklaim untuk kompensasi keterlambatan.', 'SETTLED', 1200000, 900000, 'BoJwKInShRRtlOHKzdul5D7aNH5IaRhx', NOW() - INTERVAL '18 days');

-- Checklist templates (per-season, covering every category) and per-pilgrim
-- completion state — deliberately mixed (some pilgrims fully done, some
-- partial) so the Persiapan dashboard shows real progress variance.
INSERT INTO checklist_templates (id, operator_id, season_id, title, description, category, is_required, sort_order)
VALUES
  ('00000000-0000-4000-8000-000000002301', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Paspor masih berlaku >= 6 bulan', 'Cek masa berlaku paspor sebelum keberangkatan', 'DOCUMENT',     true,  1),
  ('00000000-0000-4000-8000-000000002302', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Vaksin Meningitis', 'Sertifikat vaksin meningitis wajib', 'MEDICAL',      true,  2),
  ('00000000-0000-4000-8000-000000002303', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Pelunasan Biaya Paket', 'Pembayaran lunas sebelum H-14', 'PAYMENT',      true,  3),
  ('00000000-0000-4000-8000-000000002304', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000101', 'Manasik Haji', 'Menghadiri minimal 2x sesi manasik', 'PREPARATION',  false, 4);

INSERT INTO pilgrim_checklist_items (id, template_id, pilgrim_id, operator_id, is_completed, completed_at, completed_by, notes)
VALUES
  ('00000000-0000-4000-8000-000000002401', '00000000-0000-4000-8000-000000002301', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000001', true,  NOW() - INTERVAL '10 days', 'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x', ''),
  ('00000000-0000-4000-8000-000000002402', '00000000-0000-4000-8000-000000002302', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000001', true,  NOW() - INTERVAL '9 days',  'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x', ''),
  ('00000000-0000-4000-8000-000000002403', '00000000-0000-4000-8000-000000002303', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000001', true,  NOW() - INTERVAL '5 days',  'DZStZ3I0bz3bDHCPBsZM2G4zm49Txe1x', ''),
  ('00000000-0000-4000-8000-000000002404', '00000000-0000-4000-8000-000000002304', '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000001', false, NULL,                        '',                                    ''),
  ('00000000-0000-4000-8000-000000002405', '00000000-0000-4000-8000-000000002301', '00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000001', true,  NOW() - INTERVAL '8 days',  '8jhZGIEm9tbrn0ofO5OUVSNkWekeWZi5', ''),
  ('00000000-0000-4000-8000-000000002406', '00000000-0000-4000-8000-000000002302', '00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000001', false, NULL,                        '',                                    'Menunggu jadwal vaksin ulang.'),
  ('00000000-0000-4000-8000-000000002407', '00000000-0000-4000-8000-000000002303', '00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000001', false, NULL,                        '',                                    'Baru DP.');

-- One lost-pilgrim report — matches sos_alert 1601 (same pilgrim, ACTIVE),
-- demonstrating the two features can co-occur for the same incident without
-- being the same table.
INSERT INTO lost_reports (id, pilgrim_id, operator_id, group_id, latitude, longitude, last_known_location, status, resolved_at, created_at)
VALUES
  ('00000000-0000-4000-8000-000000002501', '00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000202', 21.4225, 39.8262, 'Sekitar Masjidil Haram, pintu King Fahd', 'LOST', NULL, NOW() - INTERVAL '2 minutes');

COMMIT;
