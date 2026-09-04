package repository

import (
	"context"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

type AddonRepository struct {
	queries *db.Queries
}

func NewAddonRepository(queries *db.Queries) *AddonRepository {
	return &AddonRepository{queries: queries}
}

func toAddonItem(row db.AddonItem) *domain.AddonItem {
	return &domain.AddonItem{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
		Name: row.Name, UnitPriceIDR: row.UnitPriceIdr, IsActive: row.IsActive, CreatedAt: row.CreatedAt.Time,
	}
}

func (r *AddonRepository) CreateItem(ctx context.Context, operatorID, seasonID, name string, unitPriceIDR int64) (*domain.AddonItem, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateAddonItem(ctx, db.CreateAddonItemParams{
		OperatorID: op, SeasonID: season, Name: name, UnitPriceIdr: unitPriceIDR,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toAddonItem(row), nil
}

func (r *AddonRepository) UpdateItem(ctx context.Context, operatorID, id, name string, unitPriceIDR int64, isActive bool) (*domain.AddonItem, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	itemID, err := pgUUID(id)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.UpdateAddonItem(ctx, db.UpdateAddonItemParams{
		ID: itemID, OperatorID: op, Name: name, UnitPriceIdr: unitPriceIDR, IsActive: isActive,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toAddonItem(row), nil
}

func (r *AddonRepository) ListItems(ctx context.Context, operatorID, seasonID string) ([]*domain.AddonItem, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	rows, err := r.queries.ListAddonItems(ctx, db.ListAddonItemsParams{OperatorID: op, SeasonID: season})
	if err != nil {
		return nil, databaseError(err)
	}
	items := make([]*domain.AddonItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toAddonItem(row))
	}
	return items, nil
}

func (r *AddonRepository) AssignPilgrimAddon(ctx context.Context, operatorID, pilgrimID, addonItemID string, quantity int32, unitPriceIDR int64, notes string) (*domain.PilgrimAddon, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	pilgrim, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	item, err := pgUUID(addonItemID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if _, err := r.queries.AssignPilgrimAddon(ctx, db.AssignPilgrimAddonParams{
		OperatorID: op, PilgrimID: pilgrim, AddonItemID: item, Quantity: quantity, UnitPriceIdr: unitPriceIDR, Notes: notes,
	}); err != nil {
		return nil, databaseError(err)
	}
	return r.findAssignment(ctx, operatorID, pilgrimID, addonItemID)
}

// findAssignment re-reads one assignment through ListPilgrimAddons, which is
// the only query carrying the joined display fields (pilgrim/addon/group
// name, computed total) that the plain upsert/update queries do not return.
func (r *AddonRepository) findAssignment(ctx context.Context, operatorID, pilgrimID, addonItemID string) (*domain.PilgrimAddon, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	item, err := pgUUID(addonItemID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	itemRow, err := r.queries.GetAddonItem(ctx, db.GetAddonItemParams{ID: item, OperatorID: op})
	if err != nil {
		return nil, databaseError(err)
	}
	rows, err := r.queries.ListPilgrimAddons(ctx, db.ListPilgrimAddonsParams{OperatorID: op, SeasonID: itemRow.SeasonID})
	if err != nil {
		return nil, databaseError(err)
	}
	for _, row := range rows {
		if uuidString(row.PilgrimID) == pilgrimID && uuidString(row.AddonItemID) == addonItemID {
			return toPilgrimAddon(row), nil
		}
	}
	return nil, apperror.ErrNotFound
}

func toPilgrimAddon(row db.ListPilgrimAddonsRow) *domain.PilgrimAddon {
	return &domain.PilgrimAddon{
		ID: uuidString(row.ID), PilgrimID: uuidString(row.PilgrimID), PilgrimName: row.PilgrimName,
		AddonItemID: uuidString(row.AddonItemID), AddonName: row.AddonName, GroupName: row.GroupName,
		Quantity: row.Quantity, UnitPriceIDR: row.UnitPriceIdr, TotalIDR: row.TotalIdr,
		Paid: row.Paid, Notes: row.Notes, CreatedAt: row.CreatedAt.Time,
	}
}

func (r *AddonRepository) SetPaid(ctx context.Context, operatorID, pilgrimAddonID string, paid bool) (*domain.PilgrimAddon, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	id, err := pgUUID(pilgrimAddonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.SetPilgrimAddonPaid(ctx, db.SetPilgrimAddonPaidParams{ID: id, OperatorID: op, Paid: paid})
	if err != nil {
		return nil, databaseError(err)
	}
	return r.findAssignment(ctx, operatorID, uuidString(row.PilgrimID), uuidString(row.AddonItemID))
}

func (r *AddonRepository) Remove(ctx context.Context, operatorID, pilgrimAddonID string) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	id, err := pgUUID(pilgrimAddonID)
	if err != nil {
		return apperror.ErrValidation
	}
	if err := r.queries.DeletePilgrimAddon(ctx, db.DeletePilgrimAddonParams{ID: id, OperatorID: op}); err != nil {
		return databaseError(err)
	}
	return nil
}

func (r *AddonRepository) ListPilgrimAddons(ctx context.Context, operatorID, seasonID, groupID string) ([]*domain.PilgrimAddon, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(seasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	var groupFilter pgtype.UUID
	if groupID != "" {
		groupFilter, err = pgUUID(groupID)
		if err != nil {
			return nil, apperror.ErrValidation
		}
	}
	rows, err := r.queries.ListPilgrimAddons(ctx, db.ListPilgrimAddonsParams{OperatorID: op, SeasonID: season, GroupID: groupFilter})
	if err != nil {
		return nil, databaseError(err)
	}
	addons := make([]*domain.PilgrimAddon, 0, len(rows))
	for _, row := range rows {
		addons = append(addons, toPilgrimAddon(row))
	}
	return addons, nil
}
