package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	"github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type InquiryRepository struct {
	queries *db.Queries
}

func NewInquiryRepository(queries *db.Queries) *InquiryRepository {
	return &InquiryRepository{queries: queries}
}

func inquiryDomain(row db.StorefrontInquiry) *domain.StorefrontInquiry {
	return &domain.StorefrontInquiry{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID),
		FullName: row.FullName, Phone: row.Phone, Email: row.Email, Message: row.Message,
		UTMSource: row.UtmSource, UTMCampaign: row.UtmCampaign, Status: row.Status,
		ConvertedLeadID: nullableUUIDString(row.ConvertedLeadID), CreatedAt: row.CreatedAt.Time,
	}
}

func (r *InquiryRepository) Create(ctx context.Context, operatorID, fullName, phone, email, message, utmSource, utmCampaign string) (*domain.StorefrontInquiry, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.CreateStorefrontInquiry(ctx, db.CreateStorefrontInquiryParams{
		OperatorID: op, FullName: strings.TrimSpace(fullName), Phone: strings.TrimSpace(phone),
		Email: strings.ToLower(strings.TrimSpace(email)), Message: strings.TrimSpace(message),
		UtmSource: strings.TrimSpace(utmSource), UtmCampaign: strings.TrimSpace(utmCampaign),
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return inquiryDomain(row), nil
}

func (r *InquiryRepository) List(ctx context.Context, operatorID, status string) ([]domain.StorefrontInquiry, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	var statusFilter pgtype.Text
	if strings.TrimSpace(status) != "" {
		statusFilter = pgtype.Text{String: status, Valid: true}
	}
	rows, err := r.queries.ListStorefrontInquiries(ctx, db.ListStorefrontInquiriesParams{OperatorID: op, Status: statusFilter})
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]domain.StorefrontInquiry, 0, len(rows))
	for _, row := range rows {
		result = append(result, *inquiryDomain(row))
	}
	return result, nil
}

// Get locks the row — the caller is about to either convert or dismiss it,
// and two staff clicking "Jadikan Lead" on the same inquiry at once must not
// both succeed.
func (r *InquiryRepository) Get(ctx context.Context, operatorID, inquiryID string) (*domain.StorefrontInquiry, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	id, err := pgUUID(inquiryID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	row, err := r.queries.LockStorefrontInquiry(ctx, db.LockStorefrontInquiryParams{ID: id, OperatorID: op})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, databaseError(err)
	}
	return inquiryDomain(row), nil
}

func (r *InquiryRepository) MarkConverted(ctx context.Context, operatorID, inquiryID, leadID string) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	id, err := pgUUID(inquiryID)
	if err != nil {
		return apperror.ErrValidation
	}
	lead, err := pgUUID(leadID)
	if err != nil {
		return apperror.ErrValidation
	}
	if err := r.queries.MarkStorefrontInquiryConverted(ctx, db.MarkStorefrontInquiryConvertedParams{ID: id, OperatorID: op, ConvertedLeadID: lead}); err != nil {
		return databaseError(err)
	}
	return nil
}

func (r *InquiryRepository) MarkDismissed(ctx context.Context, operatorID, inquiryID string) error {
	op, err := pgUUID(operatorID)
	if err != nil {
		return apperror.ErrValidation
	}
	id, err := pgUUID(inquiryID)
	if err != nil {
		return apperror.ErrValidation
	}
	if err := r.queries.MarkStorefrontInquiryDismissed(ctx, db.MarkStorefrontInquiryDismissedParams{ID: id, OperatorID: op}); err != nil {
		return databaseError(err)
	}
	return nil
}
