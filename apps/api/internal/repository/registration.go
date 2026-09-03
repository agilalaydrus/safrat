package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
)

type RegistrationRepository struct{ queries *db.Queries }

func NewRegistrationRepository(queries *db.Queries) *RegistrationRepository {
	return &RegistrationRepository{queries: queries}
}

func (r *RegistrationRepository) Create(ctx context.Context, operatorID, seasonID, productID, fullName, passportNumber string, dateOfBirth *time.Time, gender, phone, email, nationality, address, agentID, utmSource, utmCampaign string) (*domain.PilgrimRegistration, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreatePilgrimRegistration(ctx, db.CreatePilgrimRegistrationParams{
		OperatorID: opUUID, SeasonID: seasonUUID, Column3: productID, FullName: fullName, PassportNumber: passportNumber,
		DateOfBirth: pgDate(dateOfBirth), Gender: gender, Phone: phone, Email: email, Nationality: nationality, Address: address,
		AgentID:   pgUUIDOrNull(agentID),
		UtmSource: utmSource, UtmCampaign: utmCampaign,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toRegistration(row), nil
}

func (r *RegistrationRepository) List(ctx context.Context, operatorID, seasonID string) ([]*domain.PilgrimRegistration, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListPilgrimRegistrations(ctx, db.ListPilgrimRegistrationsParams{OperatorID: opUUID, SeasonID: seasonUUID, BranchScope: scope})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.PilgrimRegistration, 0, len(rows))
	for _, row := range rows {
		reg := toRegistration(db.PilgrimRegistration{
			ID: row.ID, OperatorID: row.OperatorID, SeasonID: row.SeasonID, ProductID: row.ProductID,
			FullName: row.FullName, PassportNumber: row.PassportNumber, DateOfBirth: row.DateOfBirth, Gender: row.Gender,
			Phone: row.Phone, Email: row.Email, Nationality: row.Nationality, Address: row.Address,
			Status: row.Status, Notes: row.Notes, CreatedAt: row.CreatedAt, AgentID: row.AgentID,
		})
		reg.AgentName = row.AgentName
		result = append(result, reg)
	}
	return result, nil
}

func (r *RegistrationRepository) Get(ctx context.Context, operatorID, registrationID string) (*domain.PilgrimRegistration, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	regUUID, err := pgUUID(registrationID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetPilgrimRegistration(ctx, db.GetPilgrimRegistrationParams{ID: regUUID, OperatorID: opUUID, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	return toRegistration(row), nil
}

func (r *RegistrationRepository) UpdateStatus(ctx context.Context, operatorID, registrationID, status, notes string) (*domain.PilgrimRegistration, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	regUUID, err := pgUUID(registrationID)
	if err != nil {
		return nil, err
	}
	scope, err := branchScope(ctx, r.queries, opUUID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.UpdateRegistrationStatus(ctx, db.UpdateRegistrationStatusParams{ID: regUUID, OperatorID: opUUID, Status: status, Notes: notes, BranchScope: scope})
	if err != nil {
		return nil, databaseError(err)
	}
	return toRegistration(row), nil
}

// GetOperatorSeasonInfo validates that operatorID+seasonID form a real season
// which has not ended before the public registration form is shown or
// accepted. is_active identifies the current operational season; it does not
// close registration for other future packages advertised by the storefront.
// A stale/ended/mismatched pair is treated as not found, not as a 500, since
// this is reachable by anyone with the link.
func (r *RegistrationRepository) GetOperatorSeasonInfo(ctx context.Context, operatorID, seasonID string) (operatorName, seasonName string, err error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return "", "", err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return "", "", err
	}
	row, err := r.queries.GetOperatorSeasonForRegistration(ctx, db.GetOperatorSeasonForRegistrationParams{ID: opUUID, ID_2: seasonUUID})
	if err != nil {
		return "", "", databaseError(err)
	}
	return row.OperatorName, row.SeasonName, nil
}

func (r *RegistrationRepository) ListActiveProductNames(ctx context.Context, operatorID, seasonID string) ([]string, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	seasonUUID, err := pgUUID(seasonID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListActiveProductsForRegistration(ctx, db.ListActiveProductsForRegistrationParams{OperatorID: opUUID, SeasonID: seasonUUID})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	return names, nil
}

func toRegistration(row db.PilgrimRegistration) *domain.PilgrimRegistration {
	return &domain.PilgrimRegistration{
		ID:             uuid.UUID(row.ID.Bytes).String(),
		OperatorID:     uuid.UUID(row.OperatorID.Bytes).String(),
		SeasonID:       uuid.UUID(row.SeasonID.Bytes).String(),
		ProductID:      nullableUUIDString(row.ProductID),
		FullName:       row.FullName,
		PassportNumber: row.PassportNumber,
		DateOfBirth:    datePtr(row.DateOfBirth),
		Gender:         row.Gender,
		Phone:          row.Phone,
		Email:          row.Email,
		Nationality:    row.Nationality,
		Address:        row.Address,
		Status:         row.Status,
		Notes:          row.Notes,
		CreatedAt:      row.CreatedAt.Time,
		AgentID:        nullableUUIDString(row.AgentID),
	}
}
