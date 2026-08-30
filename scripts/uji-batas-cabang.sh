#!/usr/bin/env bash
# Menguji pengaman hierarki cabang (migrasi 128) terhadap skema yang sedang
# berjalan. Read-only: setiap uji berjalan di dalam transaksi yang di-ROLLBACK.
#
# Uji ini ada karena pengaman yang tidak pernah dijalankan adalah pengaman yang
# tidak diketahui rusak. Menjalankannya harus menghasilkan 11 baris "DITOLAK"
# atau "DITERIMA" — satu pun yang berubah jadi "MASALAH" berarti ada lubang.
#
#   DATABASE_URL=... bash scripts/uji-batas-cabang.sh
set -uo pipefail
: "${DATABASE_URL:?DATABASE_URL belum diset}"

OP_A=11111111-1111-1111-1111-111111111111
OP_B=22222222-2222-2222-2222-222222222222
BR_1=aaaaaaaa-0000-0000-0000-000000000001
BR_2=aaaaaaaa-0000-0000-0000-000000000002
BR_X=bbbbbbbb-0000-0000-0000-000000000001
SEASON=cccccccc-0000-0000-0000-000000000001

SEED="INSERT INTO operators (id,better_auth_org_id,name,country,email) VALUES ('$OP_A','org-uji-a','Uji A','ID','a@uji.test'),('$OP_B','org-uji-b','Uji B','ID','b@uji.test');
INSERT INTO branches (id,operator_id,name,city) VALUES ('$BR_1','$OP_A','Bandung','Bandung'),('$BR_2','$OP_A','Medan','Medan'),('$BR_X','$OP_B','Tetangga','Solo');
INSERT INTO seasons (id,operator_id,name,type,start_date,end_date) VALUES ('$SEASON','$OP_A','Musim Uji','UMRAH_REGULER','2027-01-01','2027-01-14');"
P="INSERT INTO pilgrims (operator_id,season_id,full_name,passport_number,nationality,date_of_birth,gender"
V="VALUES ('$OP_A','$SEASON','Uji','X1','ID','1990-01-01','MALE'"

fail=0
run() { psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -qt -c "BEGIN; $SEED $1 ROLLBACK;" 2>&1; }
tolak() { out=$(run "$2"); if [ $? -ne 0 ]; then printf "  DITOLAK  %s\n" "$1"; else printf "  MASALAH  %s — LOLOS padahal harus ditolak\n" "$1"; fail=1; fi; }
terima() { out=$(run "$2"); if [ $? -eq 0 ]; then printf "  DITERIMA %s\n" "$1"; else printf "  MASALAH  %s — DITOLAK padahal harus boleh: %s\n" "$1" "$(echo "$out" | grep -m1 ERROR)"; fail=1; fi; }

echo "Harus ditolak:"
tolak "nama cabang ganda dalam satu operator"    "INSERT INTO branches (operator_id,name) VALUES ('$OP_A','  bandung  ');"
tolak "satu orang memimpin dua cabang"           "INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ('u1','$BR_1','$OP_A'),('u1','$BR_2','$OP_A');"
tolak "jamaah ditaut ke cabang operator lain"    "$P,branch_id) $V,'$BR_X');"
tolak "jamaah dipindah ke cabang operator lain"  "$P,branch_id) $V,'$BR_1'); UPDATE pilgrims SET branch_id='$BR_X' WHERE passport_number='X1';"
tolak "nama cabang kosong"                       "INSERT INTO branches (operator_id,name) VALUES ('$OP_A','   ');"
tolak "target jamaah negatif"                    "INSERT INTO branches (operator_id,name,target_pilgrims) VALUES ('$OP_A','Baru',-1);"
tolak "hapus cabang yang masih punya jamaah"     "$P,branch_id) $V,'$BR_1'); DELETE FROM branches WHERE id='$BR_1';"

echo "Harus diterima:"
terima "jamaah tanpa cabang (milik pusat)"       "$P) $V);"
terima "jamaah di cabang operatornya sendiri"    "$P,branch_id) $V,'$BR_1');"
terima "pindah jamaah antar cabang sendiri"      "$P,branch_id) $V,'$BR_1'); UPDATE pilgrims SET branch_id='$BR_2' WHERE passport_number='X1';"
terima "hapus cabang yang sudah kosong"          "DELETE FROM branches WHERE id='$BR_2';"

echo
if [ "$fail" -eq 0 ]; then echo "Semua pengaman cabang bekerja."; else echo "ADA PENGAMAN YANG BOCOR — jangan lanjut."; exit 1; fi
