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
	"github.com/jackc/pgx/v5/pgxpool"
)

type InstallmentRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewInstallmentRepository(pool *pgxpool.Pool) *InstallmentRepository {
	return &InstallmentRepository{pool: pool, queries: db.New(pool)}
}

func (r *InstallmentRepository) CreatePlan(ctx context.Context, operatorID, userID string, draft domain.InstallmentPlanDraft, schedule []domain.InstallmentScheduleDraft) (*domain.InstallmentPlanDetail, bool, error) {
	op, err := pgUUID(operatorID)
	if err != nil || strings.TrimSpace(userID) == "" || len(schedule) == 0 {
		return nil, false, apperror.ErrValidation
	}
	pilgrim, err := pgUUID(draft.PilgrimID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.queries.WithTx(tx)
	scope, err := branchScope(ctx, q, op)
	if err != nil {
		return nil, false, err
	}
	subject, err := q.GetPilgrimInstallmentSubject(ctx, db.GetPilgrimInstallmentSubjectParams{
		ID: pilgrim, OperatorID: op, BranchScope: scope,
	})
	if err != nil {
		return nil, false, databaseError(err)
	}

	plan, err := q.InsertInstallmentPlan(ctx, db.InsertInstallmentPlanParams{
		OperatorID: op, SeasonID: subject.SeasonID, PilgrimID: subject.ID, BranchID: subject.BranchID,
		Scheme: draft.Scheme, GrossAmountIdr: draft.GrossAmountIDR, CashBonusIdr: draft.CashBonusIDR,
		FirstDueDate: pgDate(&draft.FirstDueDate), CreatedByUserID: userID, IdempotencyKey: draft.IdempotencyKey,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, existingErr := q.GetInstallmentPlanByIdempotency(ctx, db.GetInstallmentPlanByIdempotencyParams{
			OperatorID: op, IdempotencyKey: draft.IdempotencyKey,
		})
		if existingErr != nil {
			return nil, false, databaseError(existingErr)
		}
		if !sameInstallmentPlanRequest(existing, subject.ID, draft) {
			return nil, false, apperror.ErrConflict
		}
		if err := tx.Rollback(ctx); err != nil {
			return nil, false, err
		}
		detail, detailErr := r.GetPlanByID(ctx, operatorID, uuidString(existing.ID))
		if detailErr != nil {
			return nil, false, detailErr
		}
		if !sameInstallmentSchedule(detail.Installments, schedule) {
			return nil, false, apperror.ErrConflict
		}
		return detail, false, nil
	}
	if err != nil {
		return nil, false, installmentDatabaseError(err)
	}
	for _, item := range schedule {
		if item.Number <= 0 || item.AmountDueIDR <= 0 || strings.TrimSpace(item.Label) == "" {
			return nil, false, apperror.ErrValidation
		}
		if _, err := q.InsertInstallment(ctx, db.InsertInstallmentParams{
			PlanID: plan.ID, OperatorID: op, BranchID: subject.BranchID,
			InstallmentNumber: item.Number, Label: item.Label,
			DueDate: pgDate(&item.DueDate), AmountDueIdr: item.AmountDueIDR,
		}); err != nil {
			return nil, false, installmentDatabaseError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, installmentDatabaseError(err)
	}
	detail, err := r.GetPlanByID(ctx, operatorID, uuidString(plan.ID))
	return detail, true, err
}

func sameInstallmentPlanRequest(existing db.InstallmentPlan, pilgrimID pgtype.UUID, draft domain.InstallmentPlanDraft) bool {
	return existing.PilgrimID == pilgrimID && existing.Scheme == draft.Scheme &&
		existing.GrossAmountIdr == draft.GrossAmountIDR && existing.CashBonusIdr == draft.CashBonusIDR &&
		existing.FirstDueDate.Valid && existing.FirstDueDate.Time.Equal(draft.FirstDueDate)
}

func sameInstallmentSchedule(existing []domain.Installment, requested []domain.InstallmentScheduleDraft) bool {
	if len(existing) != len(requested) {
		return false
	}
	for index := range existing {
		if existing[index].Number != requested[index].Number ||
			existing[index].Label != requested[index].Label ||
			existing[index].AmountDueIDR != requested[index].AmountDueIDR ||
			!existing[index].DueDate.Equal(requested[index].DueDate) {
			return false
		}
	}
	return true
}

func (r *InstallmentRepository) GetPlanByPilgrim(ctx context.Context, operatorID, pilgrimID string) (*domain.InstallmentPlanDetail, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	pilgrim, err := pgUUID(pilgrimID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, op)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetActiveInstallmentPlanByPilgrim(ctx, db.GetActiveInstallmentPlanByPilgrimParams{
		OperatorID: op, PilgrimID: pilgrim, BranchScope: scope,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	plan := domain.InstallmentPlan{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
		PilgrimID: uuidString(row.PilgrimID), PilgrimName: row.PilgrimName, BranchID: nullableUUIDString(row.BranchID),
		Scheme: row.Scheme, GrossAmountIDR: row.GrossAmountIdr, CashBonusIDR: row.CashBonusIdr,
		PayableAmountIDR: row.PayableAmountIdr.Int64, CreatedAt: row.CreatedAt.Time,
	}
	return r.loadDetail(ctx, op, scope, plan)
}

func (r *InstallmentRepository) GetPlanByID(ctx context.Context, operatorID, planID string) (*domain.InstallmentPlanDetail, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	planUUID, err := pgUUID(planID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, op)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetInstallmentPlanByID(ctx, db.GetInstallmentPlanByIDParams{
		OperatorID: op, ID: planUUID, BranchScope: scope,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	plan := domain.InstallmentPlan{
		ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
		PilgrimID: uuidString(row.PilgrimID), PilgrimName: row.PilgrimName, BranchID: nullableUUIDString(row.BranchID),
		Scheme: row.Scheme, GrossAmountIDR: row.GrossAmountIdr, CashBonusIDR: row.CashBonusIdr,
		PayableAmountIDR: row.PayableAmountIdr.Int64, CreatedAt: row.CreatedAt.Time,
	}
	return r.loadDetail(ctx, op, scope, plan)
}

func (r *InstallmentRepository) loadDetail(ctx context.Context, op, scope pgtype.UUID, plan domain.InstallmentPlan) (*domain.InstallmentPlanDetail, error) {
	planID, _ := pgUUID(plan.ID)
	rows, err := r.queries.ListInstallmentsByPlan(ctx, db.ListInstallmentsByPlanParams{
		PlanID: planID, OperatorID: op, BranchScope: scope,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	installments := make([]domain.Installment, 0, len(rows))
	for _, row := range rows {
		installments = append(installments, domain.Installment{
			ID: uuidString(row.ID), PlanID: uuidString(row.PlanID), Number: row.InstallmentNumber,
			Label: row.Label, DueDate: row.DueDate.Time, AmountDueIDR: row.AmountDueIdr,
			PaidAmountIDR: row.PaidAmountIdr, OutstandingAmountIDR: row.OutstandingAmountIdr,
			Status: row.ComputedStatus, DaysOverdue: row.DaysOverdue,
		})
		plan.PaidAmountIDR += row.PaidAmountIdr
	}
	plan.OutstandingAmountIDR = plan.PayableAmountIDR - plan.PaidAmountIDR
	plan.Status = computedPlanStatus(plan.PaidAmountIDR, plan.PayableAmountIDR, installments)
	paymentRows, err := r.queries.ListInstallmentPaymentsByPlan(ctx, db.ListInstallmentPaymentsByPlanParams{
		PlanID: planID, OperatorID: op, BranchScope: scope,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	payments := make([]domain.InstallmentPayment, 0, len(paymentRows))
	for _, row := range paymentRows {
		payments = append(payments, installmentPaymentFromDB(row))
	}
	return &domain.InstallmentPlanDetail{Plan: plan, Installments: installments, Payments: payments}, nil
}

func computedPlanStatus(paid, payable int64, installments []domain.Installment) string {
	if paid >= payable && payable > 0 {
		return "PAID"
	}
	for _, item := range installments {
		if item.Status == "OVERDUE" {
			return "OVERDUE"
		}
	}
	if paid > 0 {
		return "PARTIAL"
	}
	return "UNPAID"
}

func (r *InstallmentRepository) ListReceivables(ctx context.Context, operatorID string, filter domain.InstallmentReceivableFilter) (*domain.InstallmentReceivableResult, error) {
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	season, err := pgUUID(filter.SeasonID)
	if err != nil {
		return nil, apperror.ErrValidation
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	scope, err := branchScope(ctx, r.queries, op)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListInstallmentReceivables(ctx, db.ListInstallmentReceivablesParams{
		OperatorID: op, SeasonID: season, Column3: filter.Status, Column4: strings.TrimSpace(filter.Search),
		Limit: filter.Limit, Offset: filter.Offset, BranchScope: scope,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	result := &domain.InstallmentReceivableResult{Plans: make([]domain.InstallmentPlan, 0, len(rows))}
	for _, row := range rows {
		result.Plans = append(result.Plans, domain.InstallmentPlan{
			ID: uuidString(row.ID), OperatorID: uuidString(row.OperatorID), SeasonID: uuidString(row.SeasonID),
			PilgrimID: uuidString(row.PilgrimID), PilgrimName: row.PilgrimName, BranchID: nullableUUIDString(row.BranchID),
			Scheme: row.Scheme, GrossAmountIDR: row.GrossAmountIdr, CashBonusIDR: row.CashBonusIdr,
			PayableAmountIDR: row.PayableAmountIdr.Int64, PaidAmountIDR: row.PaidAmountIdr,
			OutstandingAmountIDR: row.OutstandingAmountIdr, Status: row.ComputedStatus, CreatedAt: row.CreatedAt.Time,
		})
	}
	result.TotalCount, err = r.queries.CountInstallmentReceivables(ctx, db.CountInstallmentReceivablesParams{
		OperatorID: op, SeasonID: season, Column3: filter.Status,
		Column4: strings.TrimSpace(filter.Search), BranchScope: scope,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	stats, err := r.queries.GetInstallmentReceivableStats(ctx, db.GetInstallmentReceivableStatsParams{
		OperatorID: op, SeasonID: season, BranchScope: scope,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	result.TotalReceivableIDR = stats.TotalReceivableIdr
	result.TotalOverdueIDR = stats.TotalOverdueIdr
	result.DueNext7DaysIDR = stats.DueNext7DaysIdr
	result.AgingCurrentIDR = stats.AgingCurrentIdr
	result.Aging1To30IDR = stats.Aging130Idr
	result.Aging31To60IDR = stats.Aging3160Idr
	result.Aging61To90IDR = stats.Aging6190Idr
	result.AgingOver90IDR = stats.AgingOver90Idr
	if stats.TotalPayableIdr > 0 {
		result.CollectionRateBPS = int32(stats.TotalPaidIdr * 10_000 / stats.TotalPayableIdr)
	}
	return result, nil
}

func (r *InstallmentRepository) RecordPayment(ctx context.Context, operatorID, userID, installmentID string, amountIDR int64, method, reference, note, idempotencyKey string) (*domain.InstallmentPayment, bool, error) {
	if amountIDR <= 0 || strings.TrimSpace(userID) == "" {
		return nil, false, apperror.ErrValidation
	}
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	installmentUUID, err := pgUUID(installmentID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, op)
	if err != nil {
		return nil, false, err
	}
	installment, err := r.queries.GetInstallmentForPayment(ctx, db.GetInstallmentForPaymentParams{
		ID: installmentUUID, OperatorID: op, BranchScope: scope,
	})
	if err != nil {
		return nil, false, databaseError(err)
	}
	row, err := r.queries.InsertInstallmentPayment(ctx, db.InsertInstallmentPaymentParams{
		PlanID: installment.PlanID, InstallmentID: installment.ID, OperatorID: op, BranchID: installment.BranchID,
		Kind: "PAYMENT", AmountIdr: amountIDR, Method: method, Reference: strings.TrimSpace(reference),
		Note: strings.TrimSpace(note), VerifiedByUserID: userID, IdempotencyKey: idempotencyKey,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, existingErr := r.queries.GetInstallmentPaymentByIdempotency(ctx, db.GetInstallmentPaymentByIdempotencyParams{
			OperatorID: op, IdempotencyKey: idempotencyKey,
		})
		if existingErr != nil {
			return nil, false, databaseError(existingErr)
		}
		if existing.Kind != "PAYMENT" || existing.InstallmentID != installment.ID || existing.AmountIdr != amountIDR ||
			existing.Method != method || existing.Reference != strings.TrimSpace(reference) || existing.Note != strings.TrimSpace(note) ||
			existing.VerifiedByUserID != userID {
			return nil, false, apperror.ErrConflict
		}
		payment := installmentPaymentFromDB(existing)
		return &payment, false, nil
	}
	if err != nil {
		return nil, false, installmentDatabaseError(err)
	}
	payment := installmentPaymentFromDB(row)
	return &payment, true, nil
}

func (r *InstallmentRepository) ReversePayment(ctx context.Context, operatorID, userID, paymentID, reason, idempotencyKey string) (*domain.InstallmentPayment, bool, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(reason) == "" {
		return nil, false, apperror.ErrValidation
	}
	op, err := pgUUID(operatorID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	paymentUUID, err := pgUUID(paymentID)
	if err != nil {
		return nil, false, apperror.ErrValidation
	}
	scope, err := branchScope(ctx, r.queries, op)
	if err != nil {
		return nil, false, err
	}
	original, err := r.queries.GetInstallmentPaymentForReversal(ctx, db.GetInstallmentPaymentForReversalParams{
		ID: paymentUUID, OperatorID: op, BranchScope: scope,
	})
	if err != nil {
		return nil, false, databaseError(err)
	}
	row, err := r.queries.InsertInstallmentPayment(ctx, db.InsertInstallmentPaymentParams{
		PlanID: original.PlanID, InstallmentID: original.InstallmentID, OperatorID: op, BranchID: original.BranchID,
		Kind: "REVERSAL", AmountIdr: -original.AmountIdr, Method: original.Method,
		Reference: original.ReceiptNumber, Note: strings.TrimSpace(reason), OriginalPaymentID: original.ID,
		VerifiedByUserID: userID, IdempotencyKey: idempotencyKey,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, existingErr := r.queries.GetInstallmentPaymentByIdempotency(ctx, db.GetInstallmentPaymentByIdempotencyParams{
			OperatorID: op, IdempotencyKey: idempotencyKey,
		})
		if existingErr != nil {
			return nil, false, databaseError(existingErr)
		}
		if existing.Kind != "REVERSAL" || existing.OriginalPaymentID != original.ID || existing.AmountIdr != -original.AmountIdr ||
			existing.Note != strings.TrimSpace(reason) || existing.VerifiedByUserID != userID {
			return nil, false, apperror.ErrConflict
		}
		payment := installmentPaymentFromDB(existing)
		return &payment, false, nil
	}
	if err != nil {
		return nil, false, installmentDatabaseError(err)
	}
	payment := installmentPaymentFromDB(row)
	return &payment, true, nil
}

func installmentPaymentFromDB(row db.InstallmentPaymentEntry) domain.InstallmentPayment {
	return domain.InstallmentPayment{
		ID: uuidString(row.ID), PlanID: uuidString(row.PlanID), InstallmentID: uuidString(row.InstallmentID),
		Kind: row.Kind, AmountIDR: row.AmountIdr, Method: row.Method, Reference: row.Reference, Note: row.Note,
		OriginalPaymentID: nullableUUIDString(row.OriginalPaymentID), VerifiedByUserID: row.VerifiedByUserID,
		ReceiptNumber: row.ReceiptNumber, CreatedAt: row.CreatedAt.Time,
	}
}

func installmentDatabaseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.ErrNotFound
	}
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23514" {
		return apperror.ErrFailedPrecondition
	}
	return databaseError(err)
}
