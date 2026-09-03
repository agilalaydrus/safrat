package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/jackc/pgx/v5"
)

// RoomTier is one way of selling the same package.
type RoomTier struct {
	Tier          string
	PriceDeltaIDR int64
	SeatQuota     *int32
	SeatsTaken    int32
	IsActive      bool
}

// roomTierOrder is the ladder as a person reads it: most people to a room
// first, because that is the cheapest and the one most packages lead with.
var roomTierOrder = map[string]int{"QUAD": 0, "TRIPLE": 1, "DOUBLE": 2}

func validRoomTier(tier string) bool {
	_, ok := roomTierOrder[tier]
	return ok
}

// ListRoomTiers returns the tiers for one product, scoped to the operator that
// owns it. The scoping is here and not in the service for the same reason it is
// everywhere else: a caller that forgets must get nothing rather than
// everything.
func (r *ProductRepository) ListRoomTiers(ctx context.Context, operatorID, productID string) ([]RoomTier, int64, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, 0, apperror.ErrValidation
	}
	product, err := pgUUID(productID)
	if err != nil {
		return nil, 0, apperror.ErrValidation
	}

	var basePrice int64
	err = r.pool.QueryRow(ctx, `SELECT price_idr FROM products WHERE id = $1 AND operator_id = $2`,
		product, operator).Scan(&basePrice)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, apperror.ErrNotFound
	}
	if err != nil {
		return nil, 0, databaseError(err)
	}

	// Seats taken counted in the same query as the tier, so the number on the
	// screen and the number the trigger enforces cannot come from two different
	// definitions of "taken".
	rows, err := r.pool.Query(ctx, `
		SELECT t.tier, t.price_delta_idr, t.seat_quota, t.is_active,
		       COALESCE((
		         SELECT COUNT(*) FROM orders o
		         WHERE o.product_id = t.product_id AND o.room_tier = t.tier
		           AND o.status NOT IN ('CANCELLED', 'EXPIRED', 'FAILED', 'REFUNDED')
		       ), 0)::int
		FROM product_room_tiers t
		WHERE t.product_id = $1 AND t.operator_id = $2`, product, operator)
	if err != nil {
		return nil, 0, databaseError(err)
	}
	defer rows.Close()
	tiers := make([]RoomTier, 0, 3)
	for rows.Next() {
		var tier RoomTier
		if err := rows.Scan(&tier.Tier, &tier.PriceDeltaIDR, &tier.SeatQuota, &tier.IsActive, &tier.SeatsTaken); err != nil {
			return nil, 0, err
		}
		tiers = append(tiers, tier)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	sortRoomTiers(tiers)
	return tiers, basePrice, nil
}

func sortRoomTiers(tiers []RoomTier) {
	for i := 1; i < len(tiers); i++ {
		for j := i; j > 0 && roomTierOrder[tiers[j].Tier] < roomTierOrder[tiers[j-1].Tier]; j-- {
			tiers[j], tiers[j-1] = tiers[j-1], tiers[j]
		}
	}
}

// SetRoomTiers replaces the whole set for one product.
//
// Replace rather than edit-one-at-a-time: three prices that form a ladder are
// one decision, and saving them individually leaves the ladder briefly wrong —
// which is exactly the moment somebody's checkout reads it. The whole thing is
// one transaction for the same reason.
func (r *ProductRepository) SetRoomTiers(ctx context.Context, operatorID, productID string, tiers []RoomTier) ([]RoomTier, int64, error) {
	operator, err := pgUUID(operatorID)
	if err != nil {
		return nil, 0, apperror.ErrValidation
	}
	product, err := pgUUID(productID)
	if err != nil {
		return nil, 0, apperror.ErrValidation
	}
	seen := map[string]bool{}
	for _, tier := range tiers {
		if !validRoomTier(tier.Tier) || seen[tier.Tier] {
			return nil, 0, apperror.ErrValidation
		}
		if tier.SeatQuota != nil && *tier.SeatQuota < 0 {
			return nil, 0, apperror.ErrValidation
		}
		seen[tier.Tier] = true
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, 0, databaseError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var basePrice int64
	err = tx.QueryRow(ctx, `SELECT price_idr FROM products WHERE id = $1 AND operator_id = $2 FOR UPDATE`,
		product, operator).Scan(&basePrice)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, apperror.ErrNotFound
	}
	if err != nil {
		return nil, 0, databaseError(err)
	}

	// A tier priced below zero is not a discount, it is a typo — and it would
	// be sold at a negative total. Checked here because this is where the base
	// price is known; the service has only the difference.
	for _, tier := range tiers {
		if basePrice+tier.PriceDeltaIDR < 0 {
			return nil, 0, apperror.ErrValidation
		}
	}

	// A tier that has already been sold cannot be made never to have existed.
	// Removing it would leave orders pointing at a tier the catalogue denies
	// offering, and the trigger would then refuse to record their cancellation.
	rows, err := tx.Query(ctx, `
		SELECT t.tier, COALESCE((
		         SELECT COUNT(*) FROM orders o
		         WHERE o.product_id = t.product_id AND o.room_tier = t.tier
		           AND o.status NOT IN ('CANCELLED', 'EXPIRED', 'FAILED', 'REFUNDED')
		       ), 0)::int
		FROM product_room_tiers t WHERE t.product_id = $1`, product)
	if err != nil {
		return nil, 0, databaseError(err)
	}
	sold := map[string]int32{}
	for rows.Next() {
		var tier string
		var taken int32
		if err := rows.Scan(&tier, &taken); err != nil {
			rows.Close()
			return nil, 0, err
		}
		sold[tier] = taken
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	for tier, taken := range sold {
		if taken > 0 && !seen[tier] {
			return nil, 0, apperror.ErrConflict
		}
	}
	// A quota below what is already sold would make the screen claim seats are
	// free while the trigger refuses every one of them.
	for _, tier := range tiers {
		if tier.SeatQuota != nil && sold[tier.Tier] > *tier.SeatQuota {
			return nil, 0, apperror.ErrConflict
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM product_room_tiers WHERE product_id = $1 AND operator_id = $2`,
		product, operator); err != nil {
		return nil, 0, databaseError(err)
	}
	for _, tier := range tiers {
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_room_tiers (product_id, operator_id, tier, price_delta_idr, seat_quota, is_active)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			product, operator, tier.Tier, tier.PriceDeltaIDR, tier.SeatQuota, tier.IsActive); err != nil {
			return nil, 0, databaseError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 0, databaseError(err)
	}
	return r.listRoomTiersAfterWrite(ctx, operatorID, productID, basePrice)
}

func (r *ProductRepository) listRoomTiersAfterWrite(ctx context.Context, operatorID, productID string, basePrice int64) ([]RoomTier, int64, error) {
	tiers, _, err := r.ListRoomTiers(ctx, operatorID, productID)
	if err != nil {
		return nil, 0, err
	}
	return tiers, basePrice, nil
}

// TrimTierName keeps the tier comparison in one place rather than repeated at
// every call site.
func TrimTierName(tier string) string { return strings.ToUpper(strings.TrimSpace(tier)) }
