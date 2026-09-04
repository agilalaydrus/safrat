package worker

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeExportBuilder struct {
	data []byte
	err  error
}

func (f fakeExportBuilder) BuildExport(context.Context, string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.data, nil
}

type fakeExportStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
	putErr  error
	deleted []string
}

func newFakeExportStorage() *fakeExportStorage {
	return &fakeExportStorage{objects: map[string][]byte{}}
}

func (f *fakeExportStorage) PutDataExport(_ context.Context, operatorID, exportID string, data []byte) (string, error) {
	if f.putErr != nil {
		return "", f.putErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := "exports/" + operatorID + "/" + exportID + ".zip"
	f.objects[key] = data
	return key, nil
}

func (f *fakeExportStorage) DeleteDataExport(_ context.Context, objectKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, objectKey)
	f.deleted = append(f.deleted, objectKey)
	return nil
}

// The worker side of the export pipeline: a claimed job either becomes a
// downloadable file or a recorded failure, and an expired one is actually
// deleted from storage, not merely forgotten in the database.
func TestDataExportHandlerProcessesAndExpiresIntegration(t *testing.T) {
	databaseURL := os.Getenv("STOREFRONT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("STOREFRONT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	operatorID, suffix := uuid.NewString(), uuid.NewString()[:8]
	exec(`INSERT INTO operators (id,better_auth_org_id,name,country,email,slug)
		VALUES ($1,$2,'Uji Worker Ekspor','ID',$3,$4)`, operatorID, "exw-"+suffix, "exw-"+suffix+"@example.test", "exw-"+suffix)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	registry := repository.NewDataExportRepository(pool)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// A successful build: the row ends READY, and the bytes actually reach
	// storage under the key the row records.
	storage := newFakeExportStorage()
	handler := NewDataExportHandler(logger, registry, fakeExportBuilder{data: []byte("isi ekspor uji")}, storage)
	if _, err := registry.Request(ctx, operatorID, "staf-uji", "wproc-"+uuid.NewString()); err != nil {
		t.Fatalf("request: %v", err)
	}
	if err := handler.HandleProcess(ctx, nil); err != nil {
		t.Fatalf("process: %v", err)
	}
	rows, err := registry.ListForOperator(ctx, operatorID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != "READY" {
		t.Fatalf("baris setelah diproses: %+v", rows)
	}
	if _, ok := storage.objects[rows[0].ObjectKey]; !ok {
		t.Fatalf("berkas tidak tersimpan di kunci %q", rows[0].ObjectKey)
	}
	if rows[0].SizeBytes != int64(len("isi ekspor uji")) {
		t.Fatalf("ukuran tercatat %d, mau %d", rows[0].SizeBytes, len("isi ekspor uji"))
	}

	// A build failure must not leave the row stuck PROCESSING forever, and
	// must say why.
	failStorage := newFakeExportStorage()
	failHandler := NewDataExportHandler(logger, registry, fakeExportBuilder{err: errors.New("basis data jamaah tidak terbaca")}, failStorage)
	if _, err := registry.Request(ctx, operatorID, "staf-uji", "wfail-"+uuid.NewString()); err != nil {
		t.Fatalf("request kedua: %v", err)
	}
	if err := failHandler.HandleProcess(ctx, nil); err == nil {
		t.Fatal("kegagalan build tidak dilaporkan sebagai galat ke asynq — job tidak akan pernah dicoba ulang")
	}
	rowsAfterFail, err := registry.ListForOperator(ctx, operatorID, 10)
	if err != nil {
		t.Fatal(err)
	}
	var failedRow *repository.DataExportRow
	for i := range rowsAfterFail {
		if rowsAfterFail[i].Status == "FAILED" {
			failedRow = &rowsAfterFail[i]
		}
	}
	if failedRow == nil || failedRow.Error == "" {
		t.Fatalf("tidak ada baris gagal tercatat: %+v", rowsAfterFail)
	}

	// Nothing left PENDING means a third call is a no-op, not an error.
	if err := handler.HandleProcess(ctx, nil); err != nil {
		t.Fatalf("proses tanpa antrean: %v", err)
	}

	// Expiry actually deletes the object, not only the database's opinion of
	// it.
	if err := registry.MarkReady(ctx, rows[0].ID, rows[0].ObjectKey, rows[0].SizeBytes, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("mark ready kedaluwarsa: %v", err)
	}
	if err := handler.HandleExpire(ctx, nil); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if _, stillThere := storage.objects[rows[0].ObjectKey]; stillThere {
		t.Fatal("berkas kedaluwarsa masih ada di penyimpanan")
	}
	afterExpiry, err := registry.Get(ctx, operatorID, rows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterExpiry.Status == "READY" {
		t.Fatal("baris masih mengaku siap diunduh setelah berkasnya dihapus")
	}
}
