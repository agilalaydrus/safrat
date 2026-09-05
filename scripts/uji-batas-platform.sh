#!/usr/bin/env bash
# Menguji constraint §9 DESAIN-PANEL-SAAS.md langsung terhadap skema yang
# sedang berjalan. Read-only: setiap uji tabel berjalan di dalam transaksi
# yang di-ROLLBACK. Pola dan gaya sama persis dengan scripts/uji-batas-cabang.sh.
#
# Uji ini ada karena pengaman yang tidak pernah dijalankan adalah pengaman yang
# tidak diketahui rusak. Menjalankannya harus menghasilkan sederet baris
# "DITOLAK" atau "DITERIMA". Satu pun yang berubah jadi "MASALAH" berarti ada
# lubang.
#
#   DATABASE_URL=... bash scripts/uji-batas-platform.sh
set -uo pipefail
: "${DATABASE_URL:?DATABASE_URL belum diset}"

OP=33333333-3333-3333-3333-333333333333
INV=44444444-4444-4444-4444-444444444444

SEED="INSERT INTO operators (id,better_auth_org_id,name,country,email,plan) VALUES ('$OP','org-uji-platform','Uji Platform','ID','p@uji.test','PRO') ON CONFLICT (id) DO NOTHING;
INSERT INTO subscriptions (operator_id,plan,status,access_until) VALUES ('$OP','PRO','ACTIVE',NOW()+INTERVAL '30 days') ON CONFLICT (operator_id) DO NOTHING;
INSERT INTO subscription_invoices (id,operator_id,plan,channel,base_amount_idr,amount_idr,period_start,period_end,due_at)
  VALUES ('$INV','$OP','PRO','BANK_TRANSFER',100000,100000,NOW(),NOW()+INTERVAL '30 days',NOW()+INTERVAL '7 days')
  ON CONFLICT (id) DO NOTHING;"

fail=0
run() { psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -qt -c "BEGIN; $SEED $1 ROLLBACK;" 2>&1; }
tolak() {
  out=$(run "$3")
  status=$?
  if [ "$status" -ne 0 ] && grep -Fq "$2" <<<"$out"; then
    printf "  DITOLAK  %s\n" "$1"
  elif [ "$status" -eq 0 ]; then
    printf "  MASALAH  %s - LOLOS padahal harus ditolak\n" "$1"
    fail=1
  else
    printf "  MASALAH  %s - gagal karena alasan lain: %s\n" "$1" "$(grep -m1 -E 'ERROR|FATAL|psql:' <<<"$out")"
    fail=1
  fi
}
terima() {
  out=$(run "$2")
  if [ $? -eq 0 ]; then
    printf "  DITERIMA %s\n" "$1"
  else
    printf "  MASALAH  %s - DITOLAK padahal harus boleh: %s\n" "$1" "$(grep -m1 -E 'ERROR|FATAL|psql:' <<<"$out")"
    fail=1
  fi
}

if ! preflight=$(run ""); then
  printf "Gagal menyiapkan data uji: %s\n" "$(grep -m1 -E 'ERROR|FATAL|psql:' <<<"$preflight")"
  exit 1
fi

echo "Harus ditolak:"
tolak "usage_counters ganda untuk periode sama"      "usage_counters_pkey" \
  "INSERT INTO usage_counters (operator_id,metric,period_start,value) VALUES ('$OP','pilgrims','2027-01-01',5),('$OP','pilgrims','2027-01-01',9);"
tolak "metrik usage_counters tidak dikenal"          "usage_counters_metric_check" \
  "INSERT INTO usage_counters (operator_id,metric,period_start,value) VALUES ('$OP','whatsapp','2027-01-01',1);"
tolak "dunning_log ganda untuk tahap sama"           "dunning_log_pkey" \
  "INSERT INTO dunning_log (operator_id,lapsed_at,stage) VALUES ('$OP','2027-01-01','H1'),('$OP','2027-01-01','H1');"
tolak "tahap dunning_log tidak dikenal"              "dunning_log_stage_check" \
  "INSERT INTO dunning_log (operator_id,lapsed_at,stage) VALUES ('$OP','2027-01-01','H99');"
tolak "privileged_actions tanpa alasan"              "privileged_actions_reason_check" \
  "INSERT INTO privileged_actions (kind,payload,reason,requested_by,approved_by,idempotency_key) VALUES ('SUSPEND','{}','   ','u1','u1','uji-key-1');"
tolak "privileged_actions kind tidak dikenal"        "privileged_actions_kind_check" \
  "INSERT INTO privileged_actions (kind,payload,reason,requested_by,approved_by,idempotency_key) VALUES ('DO_ANYTHING','{}','alasan sah','u1','u1','uji-key-2');"
tolak "privileged_actions kunci idempoten diulang"   "privileged_actions_requested_by_kind_idempotency_key_key" \
  "INSERT INTO privileged_actions (kind,payload,reason,requested_by,approved_by,idempotency_key) VALUES ('SUSPEND','{}','alasan pertama','u1','u1','uji-key-3'),('SUSPEND','{}','alasan kedua','u1','u1','uji-key-3');"
tolak "impersonation_sessions alasan terlalu pendek" "impersonation_sessions_reason_check" \
  "INSERT INTO impersonation_sessions (admin_user_id,operator_id,token_hash,reason,expires_at,idempotency_key) VALUES ('u1','$OP',repeat('a',64),'pendek',NOW()+INTERVAL '15 minutes','imp-key-1');"
tolak "impersonation_sessions token_hash salah panjang" "impersonation_sessions_token_hash_check" \
  "INSERT INTO impersonation_sessions (admin_user_id,operator_id,token_hash,reason,expires_at,idempotency_key) VALUES ('u1','$OP','pendek','alasan yang cukup panjang untuk lolos',NOW()+INTERVAL '15 minutes','imp-key-2');"
tolak "impersonation_sessions kunci idempoten diulang" "impersonation_sessions_idempotency_key_key" \
  "INSERT INTO impersonation_sessions (admin_user_id,operator_id,token_hash,reason,expires_at,idempotency_key) VALUES ('u1','$OP',repeat('a',64),'alasan yang cukup panjang untuk lolos',NOW()+INTERVAL '15 minutes','imp-key-3'),('u1','$OP',repeat('b',64),'alasan yang cukup panjang untuk lolos',NOW()+INTERVAL '15 minutes','imp-key-3');"
tolak "impersonation_sessions berakhir sebelum mulai" "impersonation_sessions_check" \
  "INSERT INTO impersonation_sessions (admin_user_id,operator_id,token_hash,reason,started_at,expires_at,idempotency_key) VALUES ('u1','$OP',repeat('a',64),'alasan yang cukup panjang untuk lolos',NOW(),NOW()-INTERVAL '1 minute','imp-key-4');"
tolak "plan_overrides tanpa catatan"                 "plan_overrides_note_required" \
  "INSERT INTO plan_overrides (operator_id,max_pilgrims,note,updated_by) VALUES ('$OP',999,'   ','u1');"

echo "Harus diterima:"
terima "usage_counters ditimpa lewat upsert, bukan digandakan" \
  "INSERT INTO usage_counters (operator_id,metric,period_start,value) VALUES ('$OP','pilgrims','2027-01-01',5) ON CONFLICT (operator_id,metric,period_start) DO UPDATE SET value = EXCLUDED.value;"
terima "dunning_log tahap berbeda untuk lapse yang sama" \
  "INSERT INTO dunning_log (operator_id,lapsed_at,stage) VALUES ('$OP','2027-01-01','H1'),('$OP','2027-01-01','H7');"
terima "privileged_actions baris sah" \
  "INSERT INTO privileged_actions (kind,payload,reason,requested_by,approved_by,idempotency_key) VALUES ('SUSPEND','{}','alasan yang sah dan jelas','u1','u1','uji-key-ok');"
terima "impersonation_sessions baris sah" \
  "INSERT INTO impersonation_sessions (admin_user_id,operator_id,token_hash,reason,expires_at,idempotency_key) VALUES ('u1','$OP',repeat('a',64),'alasan yang cukup panjang untuk lolos',NOW()+INTERVAL '15 minutes','imp-key-ok');"

echo
echo "Peran aplikasi tidak boleh menghapus bukti (§9 DESAIN: dunning_log,"
echo "privileged_actions, dan impersonation_sessions bukan cache):"
priv() {
  allowed=$(psql "$DATABASE_URL" -tAc "SELECT has_table_privilege('safrat_app','$2','$3')" 2>/dev/null)
  if [ "$allowed" = "f" ]; then
    printf "  DITOLAK  safrat_app %s pada %s\n" "$3" "$2"
  elif [ "$allowed" = "t" ]; then
    printf "  MASALAH  safrat_app masih boleh %s pada %s\n" "$3" "$2"
    fail=1
  else
    printf "  DILEWATI %s pada %s - peran safrat_app tidak ada di database ini\n" "$3" "$2"
  fi
}
for table in dunning_log privileged_actions impersonation_sessions audit_logs; do
  priv "$table" "$table" DELETE
  priv "$table" "$table" TRUNCATE
done

echo
if [ "$fail" -eq 0 ]; then echo "Semua pengaman platform bekerja."; else echo "ADA PENGAMAN YANG BOCOR - jangan lanjut."; exit 1; fi
