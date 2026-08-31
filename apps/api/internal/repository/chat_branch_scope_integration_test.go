package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestChatRepositoryRejectsMixedBranchThreadForBranchHeadIntegration(t *testing.T) {
	url := os.Getenv("STOREFRONT_TEST_DATABASE_URL"); if url == "" { t.Skip("STOREFRONT_TEST_DATABASE_URL is not set") }
	ctx := context.Background(); pool, err := pgxpool.New(ctx, url); if err != nil { t.Fatal(err) }; t.Cleanup(pool.Close)
	op, season := uuid.NewString(), uuid.NewString(); ownGroup, mixedGroup := uuid.NewString(), uuid.NewString()
	bandung, medan := uuid.NewString(), uuid.NewString(); bandungPilgrim, medanPilgrim := uuid.NewString(), uuid.NewString(); head := "chat-head-"+uuid.NewString()
	exec := func(q string, args ...any) { t.Helper(); if _, err := pool.Exec(ctx, q, args...); err != nil { t.Fatalf("fixture: %v", err) } }
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug,plan) VALUES ($1,$2,'Chat scope','ID',$3,$4,'GROWTH')`, op, "chat-scope-"+uuid.NewString(), op[:8]+"@example.test", "chat-"+op[:8]); t.Cleanup(func(){ _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id=$1`, op) })
	exec(`INSERT INTO seasons (id,operator_id,name,type,start_date,end_date,capacity) VALUES ($1,$2,'Musim','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',20)`, season, op)
	exec(`INSERT INTO groups (id,season_id,operator_id,name,capacity) VALUES ($1,$3,$4,'Bandung',20),($2,$3,$4,'Campuran',20)`, ownGroup, mixedGroup, season, op)
	exec(`INSERT INTO branches (id,operator_id,name,city) VALUES ($1,$3,'Bandung','Bandung'),($2,$3,'Medan','Medan')`, bandung, medan, op)
	exec(`INSERT INTO "user" (id,name,email,"emailVerified") VALUES ($1,'Kepala Bandung',$2,true)`, head, head+"@example.test")
	exec(`INSERT INTO branch_members (better_auth_user_id,branch_id,operator_id) VALUES ($1,$2,$3)`, head, bandung, op)
	exec(`INSERT INTO pilgrims (id,season_id,operator_id,branch_id,group_id,full_name,passport_number,nationality,date_of_birth,gender) VALUES ($1,$3,$4,$5,$6,'Jamaah Bandung','CHT-BDG','ID','1990-01-01','MALE'),($2,$3,$4,$7,$8,'Jamaah Medan','CHT-MDN','ID','1991-01-01','FEMALE')`, bandungPilgrim, medanPilgrim, season, op, bandung, ownGroup, medan, mixedGroup)
	exec(`INSERT INTO chat_messages (operator_id,group_id,sender_pilgrim_id,body) VALUES ($1,$2,$3,'Pesan Bandung')`, op, ownGroup, bandungPilgrim)
	exec(`INSERT INTO chat_messages (operator_id,group_id,sender_pilgrim_id,body) VALUES ($1,$2,$3,'Pesan Medan')`, op, mixedGroup, medanPilgrim)
	repo := NewChatRepository(db.New(pool)); branchCtx := ContextWithStaffActor(ctx, head)
	rows, err := repo.ListByGroup(branchCtx, op, mixedGroup); if err != nil || len(rows) != 0 { t.Fatalf("thread campuran bocor: %#v (%v)", rows, err) }
	if _, err = repo.CreateFromUser(branchCtx, op, mixedGroup, head, "Kepala Bandung", "blocked", uuid.NewString()); !errors.Is(err, apperror.ErrNotFound) { t.Fatalf("kepala cabang mengirim ke grup campuran: %v", err) }
	rows, err = repo.ListByGroup(branchCtx, op, ownGroup); if err != nil || len(rows) != 1 { t.Fatalf("kepala Bandung gagal membaca thread sendiri: %#v (%v)", rows, err) }
	if _, err = repo.CreateFromUser(branchCtx, op, ownGroup, head, "Kepala Bandung", "allowed", uuid.NewString()); err != nil { t.Fatalf("kepala Bandung gagal mengirim ke thread sendiri: %v", err) }
	if _, err = repo.CreateFromPilgrim(ctx, op, mixedGroup, bandungPilgrim, "Jamaah Bandung", "wrong group", uuid.NewString()); !errors.Is(err, apperror.ErrNotFound) { t.Fatalf("jamaah mengirim ke grup yang bukan miliknya: %v", err) }
	headOffice, err := repo.ListByGroup(ctx, op, mixedGroup); if err != nil || len(headOffice) != 1 { t.Fatalf("kantor pusat tidak melihat thread lengkap: %#v (%v)", headOffice, err) }
}
