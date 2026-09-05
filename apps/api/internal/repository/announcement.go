package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AnnouncementRepository struct {
	pool *pgxpool.Pool
}

func NewAnnouncementRepository(pool *pgxpool.Pool) *AnnouncementRepository {
	return &AnnouncementRepository{pool: pool}
}

// RecipientFilter is the CRITERIA that decides who an announcement goes to —
// stored as-is in announcements.recipient_filter (JSONB) and re-evaluated
// every time recipients are resolved, never frozen into a list until the
// moment of actual send. "Jumlah penerima dihitung langsung dari data, bukan
// diperkirakan" (§10.1 DESAIN) applies at send time too: a announcement
// scheduled for next week recomputes who currently matches when it actually
// fires, not who matched when it was composed.
type RecipientFilter struct {
	Mode        string   `json:"mode"` // ALL | PLAN | TRIALING | MULTI_BRANCH | OVERDUE | MANUAL
	Plan        string   `json:"plan,omitempty"`
	OperatorIDs []string `json:"operator_ids,omitempty"`
}

var ErrAnnouncementNotFound = errors.New("announcement not found")

// ResolveRecipients turns a filter into the operator ids it currently
// matches. Five modes, each a plain read against live state — no mode caches
// or precomputes anything, since the whole point is that the count and the
// list are the same query run at two different moments.
func (r *AnnouncementRepository) ResolveRecipients(ctx context.Context, filter RecipientFilter) ([]string, error) {
	var rows pgx.Rows
	var err error
	switch strings.ToUpper(filter.Mode) {
	case "ALL":
		rows, err = r.pool.Query(ctx, `SELECT id::text FROM operators`)
	case "PLAN":
		if strings.TrimSpace(filter.Plan) == "" {
			return nil, fmt.Errorf("%w: mode PLAN butuh nama paket", apperror.ErrValidation)
		}
		rows, err = r.pool.Query(ctx, `
			SELECT o.id::text FROM operators o
			JOIN subscriptions s ON s.operator_id = o.id
			WHERE s.plan::text = $1`, filter.Plan)
	case "TRIALING":
		rows, err = r.pool.Query(ctx, `
			SELECT o.id::text FROM operators o
			JOIN subscriptions s ON s.operator_id = o.id
			WHERE s.status = 'TRIALING'`)
	case "MULTI_BRANCH":
		rows, err = r.pool.Query(ctx, `
			SELECT operator_id::text FROM branches
			WHERE is_active = true
			GROUP BY operator_id HAVING COUNT(*) > 1`)
	case "OVERDUE":
		rows, err = r.pool.Query(ctx, `
			SELECT o.id::text FROM operators o
			JOIN subscriptions s ON s.operator_id = o.id
			WHERE s.status = 'PAST_DUE'`)
	case "MANUAL":
		ids := make([]string, 0, len(filter.OperatorIDs))
		for _, id := range filter.OperatorIDs {
			if isUUIDString(id) {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("%w: mode MANUAL butuh setidaknya satu travel", apperror.ErrValidation)
		}
		rows, err = r.pool.Query(ctx, `SELECT id::text FROM operators WHERE id = ANY($1::uuid[])`, ids)
	default:
		return nil, fmt.Errorf("%w: mode penerima tidak dikenal: %s", apperror.ErrValidation, filter.Mode)
	}
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// OverlapWithRecentSends is the readiness score's most important check
// (§10.1 DESAIN): "tidak ada pengumuman lain terkirim ke penerima yang sama
// dalam 24 jam terakhir." Counts how many of the candidate recipients already
// received some other announcement inside the window — not which one, since
// the number itself is the warning.
func (r *AnnouncementRepository) OverlapWithRecentSends(ctx context.Context, operatorIDs []string, within time.Duration) (int32, error) {
	if len(operatorIDs) == 0 {
		return 0, nil
	}
	var count int32
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT d.operator_id)
		FROM announcement_deliveries d
		JOIN announcements a ON a.id = d.announcement_id
		WHERE a.sent_at IS NOT NULL AND a.sent_at > NOW() - make_interval(secs => $2)
		  AND d.operator_id = ANY($1::uuid[])`,
		operatorIDs, within.Seconds()).Scan(&count)
	return count, databaseError(err)
}

type Announcement struct {
	ID              string
	AdminUserID     string
	AdminEmail      string
	Title           string
	Body            string
	Link            string
	Channels        []string
	RecipientFilter RecipientFilter
	RecipientCount  int32
	ScheduledAt     time.Time
	SentAt          *time.Time
	CreatedAt       time.Time
	ReadCount       int32
}

// Create stores an announcement with its criteria, but does not resolve
// recipients or send anything — that only happens at dispatch, either
// immediately after Create (scheduled_at <= now) or later by the worker
// sweep. Idempotent on (admin_user_id, idempotency_key): a double-submitted
// wizard settles the same announcement rather than composing two.
func (r *AnnouncementRepository) Create(ctx context.Context, adminUserID, title, body, link string, channels []string, filter RecipientFilter, scheduledAt time.Time, idempotencyKey string) (Announcement, error) {
	if strings.TrimSpace(adminUserID) == "" || strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return Announcement{}, apperror.ErrValidation
	}
	filterJSON, err := json.Marshal(filter)
	if err != nil {
		return Announcement{}, err
	}
	if len(channels) == 0 {
		channels = []string{"IN_APP"}
	}

	var existingID string
	err = r.pool.QueryRow(ctx, `SELECT id::text FROM announcements WHERE admin_user_id = $1 AND idempotency_key = $2`,
		adminUserID, idempotencyKey).Scan(&existingID)
	switch {
	case err == nil:
		return r.Get(ctx, existingID)
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return Announcement{}, databaseError(err)
	}

	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO announcements (admin_user_id, title, body, link, channels, recipient_filter, scheduled_at, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id::text`,
		adminUserID, strings.TrimSpace(title), strings.TrimSpace(body), strings.TrimSpace(link),
		channels, filterJSON, scheduledAt, idempotencyKey).Scan(&id)
	if err != nil {
		if IsUniqueViolation(err, "announcements_admin_user_id_idempotency_key_key") {
			return r.getByIdempotencyKey(ctx, adminUserID, idempotencyKey)
		}
		return Announcement{}, databaseError(err)
	}
	return r.Get(ctx, id)
}

func (r *AnnouncementRepository) getByIdempotencyKey(ctx context.Context, adminUserID, idempotencyKey string) (Announcement, error) {
	var id string
	if err := r.pool.QueryRow(ctx, `SELECT id::text FROM announcements WHERE admin_user_id = $1 AND idempotency_key = $2`,
		adminUserID, idempotencyKey).Scan(&id); err != nil {
		return Announcement{}, databaseError(err)
	}
	return r.Get(ctx, id)
}

// DueForDispatch is the worker sweep's query: announcements whose scheduled
// moment has arrived and have not fired yet. Small and self-limiting, same
// shape as cascade_events' own unprocessed-rows index.
func (r *AnnouncementRepository) DueForDispatch(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text FROM announcements WHERE sent_at IS NULL AND scheduled_at <= NOW()`)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Dispatch resolves the announcement's own stored filter fresh, freezes that
// list into announcement_deliveries, marks the announcement sent, and — for
// operators with an email on file and EMAIL in the announcement's channels —
// enqueues one outbox event per recipient. Safe to call twice: a delivery row
// already existing for an operator is left alone (ON CONFLICT DO NOTHING),
// and an already-sent announcement is a no-op, not a re-send.
func (r *AnnouncementRepository) Dispatch(ctx context.Context, announcementID string) (Announcement, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Announcement{}, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// One dispatch at a time per announcement: the scheduled sweep and a
	// "send now" request racing on the same row must not both freeze a
	// recipient list and both enqueue email events.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "announcement-dispatch:"+announcementID); err != nil {
		return Announcement{}, databaseError(err)
	}

	var sentAt *time.Time
	var filterJSON []byte
	var title, body, link string
	var channels []string
	err = tx.QueryRow(ctx, `SELECT sent_at, recipient_filter, title, body, link, channels FROM announcements WHERE id = $1 FOR UPDATE`,
		announcementID).Scan(&sentAt, &filterJSON, &title, &body, &link, &channels)
	if errors.Is(err, pgx.ErrNoRows) {
		return Announcement{}, ErrAnnouncementNotFound
	}
	if err != nil {
		return Announcement{}, databaseError(err)
	}
	if sentAt != nil {
		if err := tx.Commit(ctx); err != nil {
			return Announcement{}, databaseError(err)
		}
		return r.Get(ctx, announcementID)
	}

	var filter RecipientFilter
	if err := json.Unmarshal(filterJSON, &filter); err != nil {
		return Announcement{}, err
	}
	recipients, err := r.ResolveRecipients(ctx, filter)
	if err != nil {
		return Announcement{}, err
	}

	if _, err := tx.Exec(ctx, `UPDATE announcements SET sent_at = NOW(), recipient_count = $2 WHERE id = $1`,
		announcementID, len(recipients)); err != nil {
		return Announcement{}, databaseError(err)
	}

	wantsEmail := false
	for _, c := range channels {
		if c == "EMAIL" {
			wantsEmail = true
		}
	}

	outbox := NewOutboxRepository(db.New(r.pool))
	for _, operatorID := range recipients {
		if _, err := tx.Exec(ctx, `
			INSERT INTO announcement_deliveries (announcement_id, operator_id)
			VALUES ($1,$2) ON CONFLICT DO NOTHING`, announcementID, operatorID); err != nil {
			return Announcement{}, databaseError(err)
		}
		if !wantsEmail {
			continue
		}
		var email string
		if err := tx.QueryRow(ctx, `SELECT email FROM operators WHERE id = $1`, operatorID).Scan(&email); err != nil {
			return Announcement{}, databaseError(err)
		}
		if strings.TrimSpace(email) == "" {
			continue
		}
		payload := domain.AnnouncementEmailPayload{AnnouncementID: announcementID, Title: title, Body: body, Link: link, Email: email}
		key := fmt.Sprintf("announcement:%s:%s", announcementID, operatorID)
		if _, err := outbox.EnqueueIdempotentTx(ctx, tx, operatorID, domain.EventAnnouncementEmail, "", key, payload); err != nil {
			return Announcement{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Announcement{}, databaseError(err)
	}
	return r.Get(ctx, announcementID)
}

func (r *AnnouncementRepository) Get(ctx context.Context, id string) (Announcement, error) {
	var a Announcement
	var filterJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT a.id::text, a.admin_user_id, COALESCE(u.email, a.admin_user_id), a.title, a.body, COALESCE(a.link,''), a.channels,
		       a.recipient_filter, a.recipient_count, a.scheduled_at, a.sent_at, a.created_at,
		       COALESCE((SELECT COUNT(*) FROM announcement_deliveries d WHERE d.announcement_id = a.id AND d.read_at IS NOT NULL), 0)
		FROM announcements a
		LEFT JOIN "user" u ON u.id = a.admin_user_id
		WHERE a.id = $1`, id).
		Scan(&a.ID, &a.AdminUserID, &a.AdminEmail, &a.Title, &a.Body, &a.Link, &a.Channels, &filterJSON,
			&a.RecipientCount, &a.ScheduledAt, &a.SentAt, &a.CreatedAt, &a.ReadCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrAnnouncementNotFound
	}
	if err != nil {
		return a, databaseError(err)
	}
	_ = json.Unmarshal(filterJSON, &a.RecipientFilter)
	return a, nil
}

// ListHistory is the platform-side "Riwayat" table: newest first, each row
// carrying its own read count so nobody has to open every announcement to
// see whether anyone read it.
func (r *AnnouncementRepository) ListHistory(ctx context.Context, limit int32) ([]Announcement, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT a.id::text, a.admin_user_id, COALESCE(u.email, a.admin_user_id), a.title, a.body, COALESCE(a.link,''), a.channels,
		       a.recipient_filter, a.recipient_count, a.scheduled_at, a.sent_at, a.created_at,
		       COALESCE((SELECT COUNT(*) FROM announcement_deliveries d WHERE d.announcement_id = a.id AND d.read_at IS NOT NULL), 0)
		FROM announcements a
		LEFT JOIN "user" u ON u.id = a.admin_user_id
		ORDER BY a.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	list := make([]Announcement, 0)
	for rows.Next() {
		var a Announcement
		var filterJSON []byte
		if err := rows.Scan(&a.ID, &a.AdminUserID, &a.AdminEmail, &a.Title, &a.Body, &a.Link, &a.Channels, &filterJSON,
			&a.RecipientCount, &a.ScheduledAt, &a.SentAt, &a.CreatedAt, &a.ReadCount); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(filterJSON, &a.RecipientFilter)
		list = append(list, a)
	}
	return list, rows.Err()
}

// OperatorAnnouncement is one row of a tenant's own inbox — only what was
// actually sent to them, joined against their own read state.
type OperatorAnnouncement struct {
	ID     string
	Title  string
	Body   string
	Link   string
	SentAt time.Time
	ReadAt *time.Time
}

// ListForOperator is the tenant-facing inbox: everything actually delivered
// to this operator (announcement_deliveries is the frozen snapshot — a
// tenant added to "ALL" after send never sees an announcement they were not
// actually part of), unsent announcements are invisible by construction
// since delivery rows only exist after Dispatch.
func (r *AnnouncementRepository) ListForOperator(ctx context.Context, operatorID string, limit int32) ([]OperatorAnnouncement, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := r.pool.Query(ctx, `
		SELECT a.id::text, a.title, a.body, COALESCE(a.link,''), a.sent_at, d.read_at
		FROM announcement_deliveries d
		JOIN announcements a ON a.id = d.announcement_id
		WHERE d.operator_id = $1 AND a.sent_at IS NOT NULL
		ORDER BY a.sent_at DESC LIMIT $2`, operator, limit)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	list := make([]OperatorAnnouncement, 0)
	for rows.Next() {
		var row OperatorAnnouncement
		var sentAt *time.Time
		if err := rows.Scan(&row.ID, &row.Title, &row.Body, &row.Link, &sentAt, &row.ReadAt); err != nil {
			return nil, err
		}
		if sentAt != nil {
			row.SentAt = *sentAt
		}
		list = append(list, row)
	}
	return list, rows.Err()
}

// MarkRead is scoped to the operator's own delivery row — WHERE operator_id
// matches is what makes this impossible to use to mark another tenant's copy
// of the same announcement as read, or to read one never actually sent to
// this operator.
func (r *AnnouncementRepository) MarkRead(ctx context.Context, operatorID, announcementID, userID string) error {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	announcement, err := pgUUID(announcementID)
	if err != nil {
		return apperror.ErrValidation
	}
	command, err := r.pool.Exec(ctx, `
		UPDATE announcement_deliveries SET read_at = NOW(), read_by_user_id = $3
		WHERE announcement_id = $1 AND operator_id = $2 AND read_at IS NULL`,
		announcement, operator, userID)
	if err != nil {
		return databaseError(err)
	}
	if command.RowsAffected() == 0 {
		// Either already read (fine, idempotent) or never actually sent to
		// this operator (not fine) — distinguish so a tenant cannot probe
		// whether an announcement_id it was never sent even exists.
		var exists bool
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM announcement_deliveries WHERE announcement_id = $1 AND operator_id = $2)`,
			announcement, operator).Scan(&exists); err != nil {
			return databaseError(err)
		}
		if !exists {
			return apperror.ErrNotFound
		}
	}
	return nil
}
