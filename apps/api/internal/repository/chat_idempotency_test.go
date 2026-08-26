package repository

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Both apps queue chat while offline and replay on reconnect, and the queue
// retries per item. A send whose response was lost after the row committed must
// resolve to the message already posted, not add another copy.
func TestChatSendIsIdempotentUnderReplayIntegration(t *testing.T) {
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

	operatorID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO operators (id, better_auth_org_id, name, country, email, slug) VALUES ($1,$2,'Chat Test','ID',$3,$4)`,
		operatorID, "chat-"+uuid.NewString(), operatorID[:8]+"@example.com", "chat-"+operatorID[:8]); err != nil {
		t.Fatalf("insert operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM operators WHERE id = $1`, operatorID) })

	seasonID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO seasons (id, operator_id, name, type, start_date, end_date, capacity) VALUES ($1,$2,'Musim Uji','UMRAH_REGULER',NOW(),NOW()+INTERVAL '30 days',10)`, seasonID, operatorID); err != nil {
		t.Fatalf("insert season: %v", err)
	}
	groupID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO groups (id, operator_id, season_id, name) VALUES ($1,$2,$3,'Rombongan Uji')`, groupID, operatorID, seasonID); err != nil {
		t.Fatalf("insert group: %v", err)
	}
	userID := "user-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, name, email, "emailVerified") VALUES ($1,'Leader Uji',$2,true)`, userID, userID+"@example.com"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID) })

	chat := NewChatRepository(db.New(pool))
	const body = "Jamaah sudah berkumpul di lobi"
	key := uuid.NewString()

	// Replays arrive concurrently when connectivity returns mid-flush.
	const attempts = 6
	var wg sync.WaitGroup
	ids := make([]string, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			message, err := chat.CreateFromUser(ctx, operatorID, groupID, userID, "Leader Uji", body, key)
			if err != nil {
				t.Errorf("attempt %d: %v", index, err)
				return
			}
			ids[index] = message.ID
		}(i)
	}
	wg.Wait()

	var stored int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM chat_messages WHERE group_id = $1`, groupID).Scan(&stored); err != nil {
		t.Fatalf("count: %v", err)
	}
	if stored != 1 {
		t.Fatalf("%d messages stored for one send, want 1", stored)
	}
	for i, id := range ids {
		if id != "" && id != ids[0] {
			t.Fatalf("attempt %d resolved to a different message: %s vs %s", i, id, ids[0])
		}
	}

	// A different key is a genuinely different message and must still post.
	if _, err := chat.CreateFromUser(ctx, operatorID, groupID, userID, "Leader Uji", body, uuid.NewString()); err != nil {
		t.Fatalf("second message: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM chat_messages WHERE group_id = $1`, groupID).Scan(&stored); err != nil {
		t.Fatalf("recount: %v", err)
	}
	if stored != 2 {
		t.Fatalf("distinct sends collapsed into %d messages, want 2", stored)
	}
}
