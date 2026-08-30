#!/usr/bin/env bash
#
# Rehearse the queries in docs/INSIDEN-DATA-PRIBADI.md.
#
# A runbook whose commands have never been run is a runbook with unknown bugs,
# and the moment you find them is the moment you have least time. The first
# version of that document referred to a column called "message" that does not
# exist; running it once caught that in seconds.
#
# Read-only. It changes nothing and can be run against production safely.

set -uo pipefail

DB="${DATABASE_URL:-}"
[[ -n "$DB" ]] || { echo "FATAL: setel DATABASE_URL dulu" >&2; exit 2; }

failed=0
check() {
  local name="$1" sql="$2"
  if psql "$DB" -v ON_ERROR_STOP=1 -tAc "$sql" >/dev/null 2>/tmp/latihan-err; then
    printf '  ok    %s\n' "$name"
  else
    printf '  GAGAL %s\n' "$name"
    sed 's/^/          /' /tmp/latihan-err >&2
    failed=$((failed + 1))
  fi
}

echo "Latihan kueri insiden data pribadi"
echo

# Each of these answers one question the 72-hour clock demands. They are run
# with placeholder values: the point is that the query is valid against the
# schema as it stands today, not that it returns rows.
check "jejak baca satu akun" "
  SELECT created_at, action, entity_id, COALESCE(metadata->>'message','')
  FROM audit_logs WHERE user_id = 'x' ORDER BY created_at LIMIT 1"

check "pembacaan KYC" "
  SELECT created_at, user_id, entity_id FROM audit_logs
  WHERE action = 'kyc_record_read' ORDER BY created_at DESC LIMIT 1"

check "jamaah dalam satu musim" "
  SELECT id, full_name, email, phone FROM pilgrims
  WHERE season_id = '00000000-0000-0000-0000-000000000000'::uuid
    AND operator_id = '00000000-0000-0000-0000-000000000000'::uuid LIMIT 1"

check "seluruh jamaah satu travel" "
  SELECT p.id, p.full_name, p.email, p.phone, o.name FROM pilgrims p
  JOIN operators o ON o.id = p.operator_id
  WHERE p.operator_id = '00000000-0000-0000-0000-000000000000'::uuid LIMIT 1"

check "tujuan pencairan tersegel" "
  SELECT id, destination_key_fingerprint FROM pilgrim_refund_payout_requests LIMIT 1"

check "mutasi bank tak dikenali" "
  SELECT id, amount_idr, description FROM bank_mutations
  WHERE status = 'UNMATCHED' LIMIT 1"

echo
if [[ "$failed" -gt 0 ]]; then
  echo "$failed kueri gagal — skema sudah bergeser."
  echo "Perbarui docs/INSIDEN-DATA-PRIBADI.md sekarang, bukan saat dibutuhkan."
  exit 1
fi
echo "Semua kueri sah terhadap skema saat ini."
