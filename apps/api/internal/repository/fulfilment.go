package repository

import (
	"context"
	"errors"
	"time"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FulfilmentRepository tracks whether a supplier actually delivered what a
// jamaah paid for.
//
// Deliberately separate from order status: "did they pay" and "did it arrive"
// are different questions, and one column carrying both makes "paid but
// undelivered" inexpressible — which is exactly the state that needs to be
// visible.
type FulfilmentRepository struct {
	pool *pgxpool.Pool
}

func NewFulfilmentRepository(pool *pgxpool.Pool) *FulfilmentRepository {
	return &FulfilmentRepository{pool: pool}
}

// Fulfilment is one order's delivery state.
type Fulfilment struct {
	ID                string
	OrderID           string
	OperatorID        string
	SupplierID        string
	SupplierName      string
	ProductName       string
	PilgrimName       string
	Status            string
	SupplierReference string
	Attempts          int32
	LastError         string
	ResolutionNote    string
	SentAt            *time.Time
	DeliveredAt       *time.Time
	CreatedAt         time.Time
}

// Open records that an order now owes a delivery, and reports whether this call
// is the one that created it.
//
// ON CONFLICT DO NOTHING rather than checking first: two workers picking up the
// same paid order both pass a check-then-act, and the result is a jamaah's
// pulsa sent twice at our expense.
// Open records that a paid order owes a delivery. kind is SHIPMENT for goods a
// person hands over and SUPPLIER for anything called out to a provider — passed
// in rather than inferred from whether supplierID is empty, because that
// inference is exactly what put equipment parcels into the supplier queue.
func (r *FulfilmentRepository) Open(ctx context.Context, orderID, operatorID, supplierID, kind string) (bool, error) {
	order, err := pgUUID(orderID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	operator, err := pgUUID(operatorID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	var supplier any
	if supplierID != "" {
		parsed, err := pgUUID(supplierID)
		if err != nil {
			return false, apperror.ErrValidation
		}
		supplier = parsed
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO order_fulfilments (order_id, operator_id, supplier_id, kind)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (order_id) DO NOTHING`, order, operator, supplier, kind)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Claim moves a fulfilment from PENDING to SENT and counts the attempt,
// returning false if somebody else already claimed it.
//
// The transition is the lock. A worker that reads PENDING and then writes SENT
// as two statements can be overtaken between them; a conditional UPDATE cannot.
func (r *FulfilmentRepository) Claim(ctx context.Context, orderID string) (bool, error) {
	id, err := pgUUID(orderID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE order_fulfilments
		SET status = 'SENT', attempts = attempts + 1, sent_at = NOW(), updated_at = NOW()
		WHERE order_id = $1 AND status = 'PENDING'`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Settle applies a supplier's answer.
//
// Only a fulfilment that is still out moves, so a redelivered callback settles
// nothing a second time, and a supplier answering after a human already
// resolved the case cannot overwrite that decision.
func (r *FulfilmentRepository) Settle(ctx context.Context, orderID, status, reference, failure string) (bool, error) {
	id, err := pgUUID(orderID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE order_fulfilments
		SET status = $2,
		    supplier_reference = CASE WHEN $3 <> '' THEN $3 ELSE supplier_reference END,
		    last_error = $4,
		    delivered_at = CASE WHEN $2 = 'DELIVERED' THEN NOW() ELSE delivered_at END,
		    updated_at = NOW()
		WHERE order_id = $1 AND status IN ('PENDING', 'SENT')`, id, status, reference, failure)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Resolve is a human deciding what a fulfilment really was. Recorded distinctly
// from a supplier's own answer, so the two are never confused later.
//
// DELIVERED is included among the states this can change, which is worth being
// explicit about. It is the only way to correct a delivery that was recorded
// but never happened — and without it, refusing to refund delivered goods would
// point at a door that does not open, trapping an operator with a jamaah's
// money and no lawful way to return it.
//
// The trade is that somebody could flip a real delivery to failed in order to
// refund it. That is why who decided and why are both stored: the act is
// permitted, but never anonymous.
func (r *FulfilmentRepository) Resolve(ctx context.Context, orderID, status, userID, note string) error {
	id, err := pgUUID(orderID)
	if err != nil {
		return apperror.ErrValidation
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE order_fulfilments
		SET status = $2, resolved_by_user_id = $3, resolution_note = $4,
		    delivered_at = CASE WHEN $2 = 'DELIVERED' THEN NOW() ELSE delivered_at END,
		    updated_at = NOW()
		WHERE order_id = $1 AND status IN ('NEEDS_REVIEW', 'SENT', 'DELIVERED', 'FAILED')`, id, status, userID, note)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrFailedPrecondition
	}
	return nil
}

// ByCallbackToken finds the supplier a callback belongs to. Tokens are unique
// across suppliers, or one supplier's token would settle another's
// transactions.
func (r *FulfilmentRepository) SupplierByCallbackToken(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", apperror.ErrUnauthorized
	}
	var id string
	err := r.pool.QueryRow(ctx,
		`SELECT id::text FROM suppliers WHERE callback_token = $1 AND status = 'ACTIVE'`, token).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.ErrUnauthorized
	}
	return id, err
}

// FindOrderByReference locates the order a callback is about, either by our own
// order id or by the supplier's reference from the original request.
func (r *FulfilmentRepository) FindOrderByReference(ctx context.Context, supplierID, reference string) (string, error) {
	supplier, err := pgUUID(supplierID)
	if err != nil {
		return "", apperror.ErrValidation
	}
	var orderID string
	err = r.pool.QueryRow(ctx, `
		SELECT f.order_id::text FROM order_fulfilments f
		WHERE f.supplier_id = $1 AND (f.order_id::text = $2 OR f.supplier_reference = $2)
		LIMIT 1`, supplier, reference).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.ErrNotFound
	}
	return orderID, err
}

// ListNeedingAttention returns fulfilments a human has to look at: a supplier
// said something unreadable, or never answered at all.
// shipmentStuckAfter is when a parcel stops being "in transit" and starts
// being lost. Domestic couriers here deliver within a week; a fortnight without
// a handover recorded means somebody needs to chase it.
const shipmentStuckAfter = 14 * 24 * time.Hour

func (r *FulfilmentRepository) ListNeedingAttention(ctx context.Context, stuckAfter time.Duration, limit int32) ([]*Fulfilment, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT f.id::text, f.order_id::text, f.operator_id::text,
		       COALESCE(f.supplier_id::text, ''), COALESCE(s.name, ''),
		       p.name, COALESCE(pil.full_name, buyer.name, ''), f.status, f.supplier_reference, f.attempts,
		       f.last_error, f.resolution_note, f.sent_at, f.delivered_at, f.created_at
		FROM order_fulfilments f
		JOIN orders o ON o.id = f.order_id
		JOIN products p ON p.id = o.product_id
		LEFT JOIN pilgrims pil ON pil.id = o.pilgrim_id
		LEFT JOIN agents buyer ON buyer.id = o.buyer_agent_id
		LEFT JOIN suppliers s ON s.id = f.supplier_id
		-- Two timescales, because the two kinds of delivery have nothing in
		-- common but a status column. A supplier answers in seconds, so an
		-- hour of silence is a fault. A courier takes days, so the same
		-- threshold would raise an alarm on every parcel in transit — every
		-- sweep, until it arrived — and an alarm that fires constantly is one
		-- people stop reading, which would cost the digital fulfilments it was
		-- built for.
		WHERE f.status = 'NEEDS_REVIEW'
		   OR (f.status = 'SENT' AND f.kind = 'SUPPLIER'
		       AND f.sent_at < NOW() - make_interval(secs => $1::int))
		   OR (f.status = 'SENT' AND f.kind = 'SHIPMENT'
		       AND f.sent_at < NOW() - make_interval(secs => $2::int))
		ORDER BY f.created_at ASC
		LIMIT $3`, int32(stuckAfter.Seconds()), int32(shipmentStuckAfter.Seconds()), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fulfilments := make([]*Fulfilment, 0)
	for rows.Next() {
		var item Fulfilment
		if err := rows.Scan(&item.ID, &item.OrderID, &item.OperatorID, &item.SupplierID, &item.SupplierName,
			&item.ProductName, &item.PilgrimName, &item.Status, &item.SupplierReference, &item.Attempts,
			&item.LastError, &item.ResolutionNote, &item.SentAt, &item.DeliveredAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		fulfilments = append(fulfilments, &item)
	}
	return fulfilments, rows.Err()
}

// CountNeedingAttention is the same question without the rows, for a sweep that
// only needs to know whether to raise an alarm.
func (r *FulfilmentRepository) CountNeedingAttention(ctx context.Context, stuckAfter time.Duration) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM order_fulfilments
		WHERE status = 'NEEDS_REVIEW'
		   OR (status = 'SENT' AND sent_at < NOW() - make_interval(secs => $1::int))`,
		int32(stuckAfter.Seconds())).Scan(&count)
	return count, err
}

// StatusFor reports an order's delivery state, or empty when nothing is owed.
func (r *FulfilmentRepository) StatusFor(ctx context.Context, orderID string) (string, error) {
	id, err := pgUUID(orderID)
	if err != nil {
		return "", apperror.ErrValidation
	}
	var status string
	err = r.pool.QueryRow(ctx, `SELECT status FROM order_fulfilments WHERE order_id = $1`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return status, err
}

// OpenMissing creates fulfilment rows for paid orders that owe a delivery and
// have none, and reports how many it had to add.
//
// This closes a gap between two writes that are not in one transaction: an
// order is marked paid, and then a fulfilment is opened. A process that dies
// between them leaves a paid order owing a delivery that nothing records — and
// because every dispatch path starts from the fulfilment row, nothing would
// ever notice. The jamaah has paid and no part of the system believes anything
// is owed.
//
// Set-based and keyed by the unique constraint, so it is safe to run
// repeatedly and cannot race the normal path into a second row.
func (r *FulfilmentRepository) OpenMissing(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO order_fulfilments (order_id, operator_id, supplier_id, kind, status, last_error)
		SELECT o.id, o.operator_id,
		       CASE WHEN p.category = 'EQUIPMENT' THEN NULL ELSE pr.supplier_id END,
		       CASE WHEN p.category = 'EQUIPMENT' THEN 'SHIPMENT' ELSE 'SUPPLIER' END,
		       -- Equipment is never a routing fault: it has no supplier to route
		       -- to and never will. It waits for a person, which is PENDING, not
		       -- NEEDS_REVIEW — a queue that fills with things nobody can fix is
		       -- a queue people stop reading.
		       CASE WHEN p.category <> 'EQUIPMENT' AND pr.supplier_id IS NULL
		            THEN 'NEEDS_REVIEW' ELSE 'PENDING' END,
		       CASE WHEN p.category <> 'EQUIPMENT' AND pr.supplier_id IS NULL
		            THEN 'Produk belum punya routing supplier aktif'
		            ELSE '' END
		FROM orders o
		JOIN products p ON p.id = o.product_id
		LEFT JOIN product_routes pr ON pr.product_id = p.id AND pr.is_active
		WHERE o.status = 'PAID'
		  AND p.category <> 'TRAVEL_PACKAGE'
		  AND NOT EXISTS (SELECT 1 FROM order_fulfilments f WHERE f.order_id = o.id)
		ON CONFLICT (order_id) DO NOTHING`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Shipment is one parcel of equipment the travel owes a person.
type Shipment struct {
	OrderID           string
	ReceiptNumber     string
	ProductName       string
	BuyerName         string
	Quantity          int32
	TotalPriceIDR     int64
	Status            string
	DeliveryMethod    string
	RecipientName     string
	RecipientPhone    string
	ShippingAddress   string
	Courier           string
	TrackingNumber    string
	HandoverRecipient string
	HandoverNote      string
	HandoverProofKey  string
	PaidAt            *time.Time
	SentAt            *time.Time
	DeliveredAt       *time.Time
}

// DestinationEditable mirrors the database trigger: once a parcel has left,
// where it went stops being a field and becomes a record.
func (s *Shipment) DestinationEditable() bool {
	return s != nil && s.Status != "SENT" && s.Status != "DELIVERED"
}

const shipmentColumns = `
	f.order_id::text, COALESCE(o.receipt_number, ''), p.name,
	COALESCE(pil.full_name, buyer.name, ''), o.quantity, o.total_price_idr,
	f.status, f.delivery_method, f.recipient_name, f.recipient_phone,
	f.shipping_address, f.courier, f.tracking_number,
	f.handover_recipient, f.handover_note, f.handover_proof_key,
	o.paid_at, f.sent_at, f.delivered_at`

func scanShipment(row interface {
	Scan(dest ...any) error
}) (*Shipment, error) {
	var s Shipment
	err := row.Scan(&s.OrderID, &s.ReceiptNumber, &s.ProductName, &s.BuyerName,
		&s.Quantity, &s.TotalPriceIDR, &s.Status, &s.DeliveryMethod,
		&s.RecipientName, &s.RecipientPhone, &s.ShippingAddress,
		&s.Courier, &s.TrackingNumber, &s.HandoverRecipient, &s.HandoverNote,
		&s.HandoverProofKey, &s.PaidAt, &s.SentAt, &s.DeliveredAt)
	return &s, err
}

// ListShipments returns the parcels this operator owes, oldest first — the
// order somebody working through them would want.
func (r *FulfilmentRepository) ListShipments(ctx context.Context, operatorID string, includeDelivered bool) ([]*Shipment, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, db.New(r.pool), operator)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+shipmentColumns+`
		FROM order_fulfilments f
		JOIN orders o ON o.id = f.order_id
		JOIN products p ON p.id = o.product_id
		LEFT JOIN pilgrims pil ON pil.id = o.pilgrim_id
		LEFT JOIN agents buyer ON buyer.id = o.buyer_agent_id
		WHERE f.kind = 'SHIPMENT' AND f.operator_id = $1
		  AND ($2::bool OR f.status <> 'DELIVERED')
		  AND ($3::uuid IS NULL OR o.branch_id = $3)
		ORDER BY o.paid_at ASC NULLS LAST, f.created_at ASC
		LIMIT 500`, operator, includeDelivered, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shipments := make([]*Shipment, 0)
	for rows.Next() {
		shipment, err := scanShipment(rows)
		if err != nil {
			return nil, err
		}
		shipments = append(shipments, shipment)
	}
	return shipments, rows.Err()
}

// GetShipment reads one, scoped to the operator so a caller cannot reach
// another tenant's parcel by sending its order id.
func (r *FulfilmentRepository) GetShipment(ctx context.Context, operatorID, orderID string) (*Shipment, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	order, err := pgUUID(orderID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, db.New(r.pool), operator)
	if err != nil {
		return nil, err
	}
	shipment, err := scanShipment(r.pool.QueryRow(ctx, `
		SELECT `+shipmentColumns+`
		FROM order_fulfilments f
		JOIN orders o ON o.id = f.order_id
		JOIN products p ON p.id = o.product_id
		LEFT JOIN pilgrims pil ON pil.id = o.pilgrim_id
		LEFT JOIN agents buyer ON buyer.id = o.buyer_agent_id
		WHERE f.kind = 'SHIPMENT' AND f.operator_id = $1 AND f.order_id = $2
		  AND ($3::uuid IS NULL OR o.branch_id = $3)`,
		operator, order, scope))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	return shipment, err
}

// SaveShipmentDestination records where a parcel goes.
//
// The status predicate is the lock, not a check the caller makes first: two
// staff saving while a third marks it sent would otherwise race, and the loser
// would silently rewrite a dispatched parcel's address. The database trigger
// refuses that too — this makes it a clean "no rows" rather than an exception.
func (r *FulfilmentRepository) SaveShipmentDestination(ctx context.Context, operatorID, orderID string, item Shipment) error {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	order, err := pgUUID(orderID)
	if err != nil {
		return apperror.ErrValidation
	}
	scope, err := branchScope(ctx, db.New(r.pool), operator)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE order_fulfilments
		SET delivery_method = $3, recipient_name = $4, recipient_phone = $5,
		    shipping_address = $6, updated_at = NOW()
		WHERE order_id = $2 AND operator_id = $1 AND kind = 'SHIPMENT'
		  AND status NOT IN ('SENT', 'DELIVERED')
		  AND EXISTS (
		    SELECT 1 FROM orders o
		    WHERE o.id = order_fulfilments.order_id
		      AND ($7::uuid IS NULL OR o.branch_id = $7)
		  )`,
		operator, order, item.DeliveryMethod, item.RecipientName,
		item.RecipientPhone, item.ShippingAddress, scope)
	if err != nil {
		return databaseError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrFailedPrecondition
	}
	return nil
}

// MarkShipmentSent moves a parcel out the door.
//
// PENDING is in the predicate so a second submission cannot re-send an already
// dispatched parcel or resurrect a delivered one. Whoever moves the row does
// the work; everyone else sees no rows.
func (r *FulfilmentRepository) MarkShipmentSent(ctx context.Context, operatorID, orderID, courier, tracking string) error {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	order, err := pgUUID(orderID)
	if err != nil {
		return apperror.ErrValidation
	}
	scope, err := branchScope(ctx, db.New(r.pool), operator)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE order_fulfilments
		SET status = 'SENT', courier = $3, tracking_number = $4,
		    sent_at = NOW(), updated_at = NOW()
		WHERE order_id = $2 AND operator_id = $1 AND kind = 'SHIPMENT'
		  AND status = 'PENDING'
		  AND EXISTS (
		    SELECT 1 FROM orders o
		    WHERE o.id = order_fulfilments.order_id
		      AND ($5::uuid IS NULL OR o.branch_id = $5)
		  )`,
		operator, order, courier, tracking, scope)
	if err != nil {
		return databaseError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrFailedPrecondition
	}
	return nil
}

// MarkShipmentHandedOver records the handover.
//
// Reachable from PENDING as well as SENT, because a jamaah collecting at the
// counter never has a dispatch step. Not reachable from DELIVERED: a handover
// recorded twice would overwrite who actually signed for it.
func (r *FulfilmentRepository) MarkShipmentHandedOver(ctx context.Context, operatorID, orderID, recipient, note, proofKey, userID string) error {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	order, err := pgUUID(orderID)
	if err != nil {
		return apperror.ErrValidation
	}
	scope, err := branchScope(ctx, db.New(r.pool), operator)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE order_fulfilments
		SET status = 'DELIVERED', handover_recipient = $3, handover_note = $4,
		    handover_proof_key = $5, handed_over_by_user_id = $6,
		    delivered_at = NOW(), updated_at = NOW()
		WHERE order_id = $2 AND operator_id = $1 AND kind = 'SHIPMENT'
		  AND status IN ('PENDING', 'SENT')
		  AND EXISTS (
		    SELECT 1 FROM orders o
		    WHERE o.id = order_fulfilments.order_id
		      AND ($7::uuid IS NULL OR o.branch_id = $7)
		  )`,
		operator, order, recipient, note, proofKey, userID, scope)
	if err != nil {
		return databaseError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrFailedPrecondition
	}
	return nil
}

// MarkShipmentLost closes out a parcel that never arrived.
//
// Reachable only from PENDING or SENT. A delivered parcel is not lost, and
// letting DELIVERED move here would let a recorded handover be undone — the
// handover is evidence somebody signed for it, and evidence should not be
// erasable by a later click.
func (r *FulfilmentRepository) MarkShipmentLost(ctx context.Context, operatorID, orderID, note, userID string) (bool, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	order, err := pgUUID(orderID)
	if err != nil {
		return false, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, db.New(r.pool), operator)
	if err != nil {
		return false, err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE order_fulfilments
		SET status = 'FAILED', last_error = $3, resolved_by_user_id = $4,
		    resolution_note = $3, updated_at = NOW()
		WHERE order_id = $2 AND operator_id = $1 AND kind = 'SHIPMENT'
		  AND status IN ('PENDING', 'SENT')
		  AND EXISTS (
		    SELECT 1 FROM orders o
		    WHERE o.id = order_fulfilments.order_id
		      AND ($5::uuid IS NULL OR o.branch_id = $5)
		  )`,
		operator, order, note, userID, scope)
	if err != nil {
		return false, databaseError(err)
	}
	return tag.RowsAffected() == 1, nil
}
