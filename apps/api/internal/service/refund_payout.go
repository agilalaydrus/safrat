package service

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RefundPayoutService struct {
	operators *repository.OperatorRepository
	identity  *repository.IdentityRepository
	payouts   *repository.RefundPayoutRepository
	ledger    *repository.LedgerRepository
	audit     *repository.AuditRepository
	db        *pgxpool.Pool
}

func NewRefundPayoutService(operators *repository.OperatorRepository, identity *repository.IdentityRepository, payouts *repository.RefundPayoutRepository, ledger *repository.LedgerRepository, audit *repository.AuditRepository, db *pgxpool.Pool) *RefundPayoutService {
	return &RefundPayoutService{operators: operators, identity: identity, payouts: payouts, ledger: ledger, audit: audit, db: db}
}

func (s *RefundPayoutService) resolveOwnPilgrim(ctx context.Context, appAccessCode, operation string) (*domain.PilgrimSummary, string, error) {
	if strings.TrimSpace(appAccessCode) == "" {
		return nil, "", serviceError(operation, apperror.ErrValidation)
	}
	userID := middleware.UserIDFromCtx(ctx)
	if userID == "" {
		return nil, "", serviceError(operation, apperror.ErrUnauthorized)
	}
	access, err := s.identity.GetMyAccess(ctx, userID)
	if err != nil {
		return nil, "", serviceError(operation, err)
	}
	if access.LinkedPilgrim == nil || subtle.ConstantTimeCompare([]byte(access.LinkedPilgrim.AppAccessCode), []byte(appAccessCode)) != 1 {
		return nil, "", serviceError(operation, apperror.ErrForbidden)
	}
	return access.LinkedPilgrim, userID, nil
}

func (s *RefundPayoutService) GetMyRefundWallet(ctx context.Context, req *hajjv1.GetMyRefundWalletRequest) (*hajjv1.RefundWallet, error) {
	const operation = "RefundPayoutService.GetMyRefundWallet"
	if req == nil {
		return nil, serviceError(operation, apperror.ErrValidation)
	}
	pilgrim, _, err := s.resolveOwnPilgrim(ctx, req.AppAccessCode, operation)
	if err != nil {
		return nil, err
	}
	balance, err := s.ledger.PilgrimBalance(ctx, pilgrim.ID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	reserved, err := s.payouts.ReservedForPilgrim(ctx, pilgrim.ID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	entries, err := s.ledger.ListPilgrimBalanceEntries(ctx, pilgrim.ID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	requests, err := s.payouts.ListForPilgrim(ctx, pilgrim.ID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	result := &hajjv1.RefundWallet{
		BalanceIdr: balance, ReservedIdr: reserved, AvailableIdr: balance - reserved,
		Entries:        make([]*hajjv1.RefundBalanceEntry, 0, len(entries)),
		PayoutRequests: make([]*hajjv1.RefundPayoutRequest, 0, len(requests)),
	}
	for _, entry := range entries {
		result.Entries = append(result.Entries, &hajjv1.RefundBalanceEntry{
			Id: entry.ID, AmountIdr: entry.AmountIDR, Kind: entry.Kind, Note: entry.Note,
			OrderId: entry.OrderID, CreatedAt: timestamppb.New(entry.CreatedAt),
		})
	}
	for _, request := range requests {
		result.PayoutRequests = append(result.PayoutRequests, refundPayoutMessage(request))
	}
	return result, nil
}

func (s *RefundPayoutService) RequestRefundPayout(ctx context.Context, req *hajjv1.RequestRefundPayoutRequest) (*hajjv1.RefundPayoutRequest, error) {
	const operation = "RefundPayoutService.RequestRefundPayout"
	if req == nil || req.AmountIdr <= 0 || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, serviceError(operation, apperror.ErrValidation)
	}
	method := refundPayoutMethodToDB(req.Method)
	if method == "" {
		return nil, serviceError(operation, apperror.ErrValidation)
	}
	pilgrim, userID, err := s.resolveOwnPilgrim(ctx, req.AppAccessCode, operation)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, serviceError(operation, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Request creation and payout completion both take this lock. The balance
	// and active reservations therefore form one serialised decision even when
	// two devices submit at the same instant.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, pilgrim.ID); err != nil {
		return nil, serviceError(operation, err)
	}
	// Replays are answered before today's preconditions. A request already
	// accepted under this key stays the same request even if the balance or 2FA
	// state changed afterwards.
	existing, err := s.payouts.FindByKeyTx(ctx, tx, pilgrim.ID, req.IdempotencyKey)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if existing != nil {
		return refundPayoutMessage(existing), nil
	}
	twoFactorEnabled, err := s.payouts.UserHasTwoFactor(ctx, userID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if !twoFactorEnabled {
		return nil, serviceError(operation, preconditionError("aktifkan verifikasi dua langkah sebelum meminta pencairan"))
	}
	balance, err := s.ledger.PilgrimBalanceTx(ctx, tx, pilgrim.ID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	reserved, err := s.payouts.ReservedForPilgrimTx(ctx, tx, pilgrim.ID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if req.AmountIdr > balance-reserved {
		return nil, serviceError(operation, preconditionError("jumlah pencairan melebihi saldo tersedia"))
	}
	operatorID, err := pilgrimOperatorID(ctx, tx, pilgrim.ID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	created, err := s.payouts.CreateTx(ctx, tx, repository.CreateRefundPayoutParams{
		OperatorID: operatorID, PilgrimID: pilgrim.ID,
		AmountIDR: req.AmountIdr, Method: method, Note: strings.TrimSpace(req.Note),
		IdempotencyKey: req.IdempotencyKey, RequestedByUserID: userID,
	})
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serviceError(operation, err)
	}
	_ = s.audit.Write(ctx, created.OperatorID, userID, "pilgrim_refund_payout_requested", "refund_payout", created.ID,
		fmt.Sprintf("%s via %s", rupiah(created.AmountIDR), created.Method))
	return refundPayoutMessage(created), nil
}

// pilgrimOperatorID is read inside the request transaction so tenant identity
// can never be supplied by a session-only caller. Empty means no such pilgrim.
func pilgrimOperatorID(ctx context.Context, tx pgx.Tx, pilgrimID string) (string, error) {
	var operatorID string
	err := tx.QueryRow(ctx, `SELECT operator_id::text FROM pilgrims WHERE id = $1::uuid`, pilgrimID).Scan(&operatorID)
	if err == pgx.ErrNoRows {
		return "", apperror.ErrNotFound
	}
	return operatorID, err
}

func (s *RefundPayoutService) ListRefundPayoutRequests(ctx context.Context, orgID string, req *hajjv1.ListRefundPayoutRequestsRequest) (*hajjv1.ListRefundPayoutRequestsResponse, error) {
	const operation = "RefundPayoutService.ListRefundPayoutRequests"
	if !operatorCanManageMoney(ctx) {
		return nil, serviceError(operation, apperror.ErrForbidden)
	}
	op, err := s.operators.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	status := ""
	if req != nil {
		status = refundPayoutStatusToDB(req.Status)
		if req.Status != hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_UNSPECIFIED && status == "" {
			return nil, serviceError(operation, apperror.ErrValidation)
		}
	}
	requests, err := s.payouts.ListByOperator(ctx, op.ID, status)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	result := &hajjv1.ListRefundPayoutRequestsResponse{Requests: make([]*hajjv1.RefundPayoutRequest, 0, len(requests))}
	for _, request := range requests {
		result.Requests = append(result.Requests, refundPayoutMessage(request))
	}
	return result, nil
}

func (s *RefundPayoutService) TransitionRefundPayout(ctx context.Context, orgID, userID string, req *hajjv1.TransitionRefundPayoutRequest) (*hajjv1.RefundPayoutRequest, error) {
	const operation = "RefundPayoutService.TransitionRefundPayout"
	if req == nil || !isUUID(req.RequestId) {
		return nil, serviceError(operation, apperror.ErrValidation)
	}
	if !operatorCanManageMoney(ctx) {
		return nil, serviceError(operation, apperror.ErrForbidden)
	}
	op, err := s.operators.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, serviceError(operation, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := s.payouts.LockByIDTx(ctx, tx, op.ID, req.RequestId)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, current.PilgrimID); err != nil {
		return nil, serviceError(operation, err)
	}

	target, action, err := validateRefundPayoutTransition(current, req)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if target == current.Status {
		return refundPayoutMessage(current), nil
	}
	if target == "PAID" {
		if err := s.ledger.AppendBalanceTx(ctx, tx, repository.BalanceEntry{
			OperatorID: current.OperatorID, PilgrimID: current.PilgrimID,
			AmountIDR: -current.AmountIDR, Kind: "WITHDRAWAL",
			Note:            "Pencairan saldo refund: " + strings.TrimSpace(req.PaymentReference),
			CreatedByUserID: userID, IdempotencyKey: "pilgrim-payout-" + current.ID,
		}); err != nil {
			return nil, serviceError(operation, err)
		}
	}
	updated, err := s.payouts.TransitionTx(ctx, tx, current.ID, target, userID,
		strings.TrimSpace(req.Note), strings.TrimSpace(req.PaymentReference))
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serviceError(operation, err)
	}
	_ = s.audit.Write(ctx, op.ID, userID, action, "refund_payout", updated.ID,
		fmt.Sprintf("%s; %s", rupiah(updated.AmountIDR), strings.TrimSpace(req.Note)))
	return refundPayoutMessage(updated), nil
}

func validateRefundPayoutTransition(current *domain.RefundPayoutRequest, req *hajjv1.TransitionRefundPayoutRequest) (target, auditAction string, err error) {
	switch req.Action {
	case hajjv1.RefundPayoutAction_REFUND_PAYOUT_ACTION_START_PROCESSING:
		if current.Status == "PROCESSING" {
			return current.Status, "pilgrim_refund_payout_processing", nil
		}
		if current.Status != "REQUESTED" {
			return "", "", preconditionError("hanya permintaan baru yang dapat mulai diproses")
		}
		return "PROCESSING", "pilgrim_refund_payout_processing", nil
	case hajjv1.RefundPayoutAction_REFUND_PAYOUT_ACTION_MARK_PAID:
		if current.Status == "PAID" {
			return current.Status, "pilgrim_refund_payout_paid", nil
		}
		if current.Status != "PROCESSING" {
			return "", "", preconditionError("mulai proses pencairan sebelum menandainya dibayar")
		}
		if strings.TrimSpace(req.PaymentReference) == "" {
			return "", "", preconditionError("referensi pembayaran wajib diisi")
		}
		return "PAID", "pilgrim_refund_payout_paid", nil
	case hajjv1.RefundPayoutAction_REFUND_PAYOUT_ACTION_MARK_FAILED:
		if current.Status == "FAILED" {
			return current.Status, "pilgrim_refund_payout_failed", nil
		}
		if current.Status != "REQUESTED" && current.Status != "PROCESSING" {
			return "", "", preconditionError("pencairan yang sudah dibayar tidak dapat digagalkan")
		}
		if strings.TrimSpace(req.Note) == "" {
			return "", "", preconditionError("alasan kegagalan wajib diisi")
		}
		return "FAILED", "pilgrim_refund_payout_failed", nil
	default:
		return "", "", apperror.ErrValidation
	}
}

func operatorCanManageMoney(ctx context.Context) bool {
	role := middleware.OrgRoleFromCtx(ctx)
	return role == "owner" || role == "admin"
}

func refundPayoutMethodToDB(method hajjv1.RefundPayoutMethod) string {
	switch method {
	case hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_BANK_TRANSFER:
		return "BANK_TRANSFER"
	case hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_EWALLET:
		return "EWALLET"
	case hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_CASH:
		return "CASH"
	default:
		return ""
	}
}

func refundPayoutMethodFromDB(method string) hajjv1.RefundPayoutMethod {
	switch method {
	case "BANK_TRANSFER":
		return hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_BANK_TRANSFER
	case "EWALLET":
		return hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_EWALLET
	case "CASH":
		return hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_CASH
	default:
		return hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_UNSPECIFIED
	}
}

func refundPayoutStatusToDB(status hajjv1.RefundPayoutStatus) string {
	switch status {
	case hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_REQUESTED:
		return "REQUESTED"
	case hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_PROCESSING:
		return "PROCESSING"
	case hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_PAID:
		return "PAID"
	case hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_FAILED:
		return "FAILED"
	default:
		return ""
	}
}

func refundPayoutStatusFromDB(status string) hajjv1.RefundPayoutStatus {
	switch status {
	case "REQUESTED":
		return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_REQUESTED
	case "PROCESSING":
		return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_PROCESSING
	case "PAID":
		return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_PAID
	case "FAILED":
		return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_FAILED
	default:
		return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_UNSPECIFIED
	}
}

func refundPayoutMessage(request *domain.RefundPayoutRequest) *hajjv1.RefundPayoutRequest {
	message := &hajjv1.RefundPayoutRequest{
		Id: request.ID, PilgrimId: request.PilgrimID, PilgrimName: request.PilgrimName,
		PilgrimPhone: request.PilgrimPhone, AmountIdr: request.AmountIDR,
		Method: refundPayoutMethodFromDB(request.Method), Note: request.Note,
		Status: refundPayoutStatusFromDB(request.Status), ResolutionNote: request.ResolutionNote,
		PaymentReference: request.PaymentReference, CreatedAt: timestamppb.New(request.CreatedAt),
	}
	if request.ProcessingAt != nil {
		message.ProcessingAt = timestamppb.New(*request.ProcessingAt)
	}
	if request.ResolvedAt != nil {
		message.ResolvedAt = timestamppb.New(*request.ResolvedAt)
	}
	return message
}
