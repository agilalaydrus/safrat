package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/payment"
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
	xendit    *payment.Client
}

func NewRefundPayoutService(operators *repository.OperatorRepository, identity *repository.IdentityRepository, payouts *repository.RefundPayoutRepository, ledger *repository.LedgerRepository, audit *repository.AuditRepository, db *pgxpool.Pool, clients ...*payment.Client) *RefundPayoutService {
	var client *payment.Client
	if len(clients) > 0 {
		client = clients[0]
	}
	return &RefundPayoutService{operators: operators, identity: identity, payouts: payouts, ledger: ledger, audit: audit, db: db, xendit: client}
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

func (s *RefundPayoutService) resolveOwnAgent(ctx context.Context, operation string) (*domain.Agent, string, error) {
	userID := middleware.UserIDFromCtx(ctx)
	if userID == "" {
		return nil, "", serviceError(operation, apperror.ErrUnauthorized)
	}
	access, err := s.identity.GetMyAccess(ctx, userID)
	if err != nil {
		return nil, "", serviceError(operation, err)
	}
	if access.LinkedAgent == nil || !access.LinkedAgent.IsActive {
		return nil, "", serviceError(operation, apperror.ErrForbidden)
	}
	return access.LinkedAgent, userID, nil
}

func (s *RefundPayoutService) GetMyRefundWallet(ctx context.Context, req *hajjv1.GetMyRefundWalletRequest) (*hajjv1.RefundWallet, error) {
	const op = "RefundPayoutService.GetMyRefundWallet"
	if req == nil {
		return nil, serviceError(op, apperror.ErrValidation)
	}
	pilgrim, _, err := s.resolveOwnPilgrim(ctx, req.AppAccessCode, op)
	if err != nil {
		return nil, err
	}
	return s.wallet(ctx, "PILGRIM", pilgrim.ID, op)
}

func (s *RefundPayoutService) GetMyAgentRefundWallet(ctx context.Context, _ *hajjv1.GetMyAgentRefundWalletRequest) (*hajjv1.RefundWallet, error) {
	const op = "RefundPayoutService.GetMyAgentRefundWallet"
	agent, _, err := s.resolveOwnAgent(ctx, op)
	if err != nil {
		return nil, err
	}
	return s.wallet(ctx, "AGENT", agent.ID, op)
}

func (s *RefundPayoutService) wallet(ctx context.Context, kind, id, operation string) (*hajjv1.RefundWallet, error) {
	var balance int64
	var entries []*domain.PilgrimBalanceEntry
	var requests []*domain.RefundPayoutRequest
	var err error
	if kind == "AGENT" {
		balance, err = s.ledger.AgentRefundBalance(ctx, id)
		if err == nil {
			entries, err = s.ledger.ListAgentRefundBalanceEntries(ctx, id)
		}
		if err == nil {
			requests, err = s.payouts.ListForAgent(ctx, id)
		}
	} else {
		balance, err = s.ledger.PilgrimBalance(ctx, id)
		if err == nil {
			entries, err = s.ledger.ListPilgrimBalanceEntries(ctx, id)
		}
		if err == nil {
			requests, err = s.payouts.ListForPilgrim(ctx, id)
		}
	}
	if err != nil {
		return nil, serviceError(operation, err)
	}
	reserved, err := s.payouts.ReservedForBeneficiary(ctx, kind, id)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	result := &hajjv1.RefundWallet{BalanceIdr: balance, ReservedIdr: reserved, AvailableIdr: balance - reserved, Entries: make([]*hajjv1.RefundBalanceEntry, 0, len(entries)), PayoutRequests: make([]*hajjv1.RefundPayoutRequest, 0, len(requests))}
	for _, entry := range entries {
		result.Entries = append(result.Entries, &hajjv1.RefundBalanceEntry{Id: entry.ID, AmountIdr: entry.AmountIDR, Kind: entry.Kind, Note: entry.Note, OrderId: entry.OrderID, CreatedAt: timestamppb.New(entry.CreatedAt)})
	}
	for _, request := range requests {
		result.PayoutRequests = append(result.PayoutRequests, refundPayoutMessage(request))
	}
	return result, nil
}

type payoutInput struct {
	amount                              int64
	method                              hajjv1.RefundPayoutMethod
	note, key, channel, holder, account string
}

func (s *RefundPayoutService) RequestRefundPayout(ctx context.Context, req *hajjv1.RequestRefundPayoutRequest) (*hajjv1.RefundPayoutRequest, error) {
	const op = "RefundPayoutService.RequestRefundPayout"
	if req == nil {
		return nil, serviceError(op, apperror.ErrValidation)
	}
	pilgrim, userID, err := s.resolveOwnPilgrim(ctx, req.AppAccessCode, op)
	if err != nil {
		return nil, err
	}
	return s.request(ctx, "PILGRIM", pilgrim.ID, userID, payoutInput{req.AmountIdr, req.Method, req.Note, req.IdempotencyKey, req.DestinationChannel, req.DestinationAccountHolder, req.DestinationAccountNumber}, op)
}

func (s *RefundPayoutService) RequestAgentRefundPayout(ctx context.Context, req *hajjv1.RequestAgentRefundPayoutRequest) (*hajjv1.RefundPayoutRequest, error) {
	const op = "RefundPayoutService.RequestAgentRefundPayout"
	if req == nil {
		return nil, serviceError(op, apperror.ErrValidation)
	}
	agent, userID, err := s.resolveOwnAgent(ctx, op)
	if err != nil {
		return nil, err
	}
	return s.request(ctx, "AGENT", agent.ID, userID, payoutInput{req.AmountIdr, req.Method, req.Note, req.IdempotencyKey, req.DestinationChannel, req.DestinationAccountHolder, req.DestinationAccountNumber}, op)
}

var accountDigits = regexp.MustCompile(`^[0-9]{7,34}$`)
var swiftCode = regexp.MustCompile(`^[A-Z0-9]{8}([A-Z0-9]{3})?$`)
var walletChannels = map[string]bool{"ID_DANA": true, "ID_OVO": true, "ID_GOPAY": true, "ID_SHOPEEPAY": true, "ID_LINKAJA": true}

func validatePayoutDestination(method string, in payoutInput) error {
	if method == "CASH" {
		return nil
	}
	channel := strings.ToUpper(strings.TrimSpace(in.channel))
	account := strings.TrimSpace(in.account)
	if strings.TrimSpace(in.holder) == "" || !accountDigits.MatchString(account) {
		return preconditionError("nama pemilik dan nomor tujuan pembayaran wajib diisi dengan benar")
	}
	if method == "BANK_TRANSFER" && !swiftCode.MatchString(channel) {
		return preconditionError("pilih kode SWIFT bank tujuan yang valid")
	}
	if method == "EWALLET" && !walletChannels[channel] {
		return preconditionError("penyedia e-wallet belum didukung")
	}
	return nil
}

func (s *RefundPayoutService) request(ctx context.Context, kind, ownerID, userID string, in payoutInput, operation string) (*hajjv1.RefundPayoutRequest, error) {
	method := refundPayoutMethodToDB(in.method)
	if in.amount <= 0 || strings.TrimSpace(in.key) == "" || method == "" {
		return nil, serviceError(operation, apperror.ErrValidation)
	}
	if err := validatePayoutDestination(method, in); err != nil {
		return nil, serviceError(operation, err)
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, serviceError(operation, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, ownerID); err != nil {
		return nil, serviceError(operation, err)
	}
	existing, err := s.payouts.FindByKeyTx(ctx, tx, kind, ownerID, in.key)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if existing != nil {
		return refundPayoutMessage(existing), nil
	}
	twoFactor, err := s.payouts.UserHasTwoFactor(ctx, userID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if !twoFactor {
		return nil, serviceError(operation, preconditionError("aktifkan verifikasi dua langkah sebelum meminta pencairan"))
	}
	var balance int64
	if kind == "AGENT" {
		balance, err = s.ledger.AgentRefundBalanceTx(ctx, tx, ownerID)
	} else {
		balance, err = s.ledger.PilgrimBalanceTx(ctx, tx, ownerID)
	}
	if err != nil {
		return nil, serviceError(operation, err)
	}
	reserved, err := s.payouts.ReservedForBeneficiaryTx(ctx, tx, kind, ownerID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if in.amount > balance-reserved {
		return nil, serviceError(operation, preconditionError("jumlah pencairan melebihi saldo tersedia"))
	}
	operatorID, err := beneficiaryOperatorID(ctx, tx, kind, ownerID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	created, err := s.payouts.CreateTx(ctx, tx, repository.CreateRefundPayoutParams{OperatorID: operatorID, BeneficiaryKind: kind, BeneficiaryID: ownerID, AmountIDR: in.amount, Method: method, Note: strings.TrimSpace(in.note), IdempotencyKey: in.key, RequestedByUserID: userID, DestinationChannel: strings.ToUpper(strings.TrimSpace(in.channel)), DestinationAccountHolder: strings.TrimSpace(in.holder), DestinationAccountNumber: strings.TrimSpace(in.account)})
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serviceError(operation, err)
	}
	_ = s.audit.Write(ctx, operatorID, userID, "refund_payout_requested", "refund_payout", created.ID, fmt.Sprintf("%s via %s untuk %s", rupiah(created.AmountIDR), created.Method, kind))
	return refundPayoutMessage(created), nil
}

func beneficiaryOperatorID(ctx context.Context, tx pgx.Tx, kind, id string) (string, error) {
	table := "pilgrims"
	if kind == "AGENT" {
		table = "agents"
	} else if kind != "PILGRIM" {
		return "", apperror.ErrValidation
	}
	var operatorID string
	err := tx.QueryRow(ctx, `SELECT operator_id::text FROM `+table+` WHERE id=$1::uuid`, id).Scan(&operatorID)
	if errors.Is(err, pgx.ErrNoRows) {
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
	for _, r := range requests {
		result.Requests = append(result.Requests, refundPayoutMessage(r))
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
	if current.Method != "CASH" {
		return nil, serviceError(operation, preconditionError("transfer bank dan e-wallet diproses otomatis oleh gateway"))
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, beneficiaryID(current)); err != nil {
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
		if err := s.appendPayoutMovement(ctx, tx, current, -current.AmountIDR, "WITHDRAWAL", "refund-payout-"+current.ID, "Pencairan tunai: "+strings.TrimSpace(req.PaymentReference), userID); err != nil {
			return nil, serviceError(operation, err)
		}
	}
	updated, err := s.payouts.TransitionTx(ctx, tx, current.ID, target, userID, strings.TrimSpace(req.Note), strings.TrimSpace(req.PaymentReference))
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serviceError(operation, err)
	}
	_ = s.audit.Write(ctx, op.ID, userID, action, "refund_payout", updated.ID, fmt.Sprintf("%s; %s", rupiah(updated.AmountIDR), strings.TrimSpace(req.Note)))
	return refundPayoutMessage(updated), nil
}

func beneficiaryID(r *domain.RefundPayoutRequest) string {
	if r.BeneficiaryKind == "AGENT" {
		return r.AgentID
	}
	return r.PilgrimID
}
func (s *RefundPayoutService) appendPayoutMovement(ctx context.Context, tx pgx.Tx, r *domain.RefundPayoutRequest, amount int64, kind, key, note, userID string) error {
	if r.BeneficiaryKind == "AGENT" {
		return s.ledger.AppendAgentRefundBalanceTx(ctx, tx, repository.AgentRefundBalanceEntry{OperatorID: r.OperatorID, AgentID: r.AgentID, AmountIDR: amount, Kind: kind, Note: note, CreatedByUserID: userID, IdempotencyKey: key})
	}
	return s.ledger.AppendBalanceTx(ctx, tx, repository.BalanceEntry{OperatorID: r.OperatorID, PilgrimID: r.PilgrimID, AmountIDR: amount, Kind: kind, Note: note, CreatedByUserID: userID, IdempotencyKey: key})
}

// DispatchGatewayPayout performs no network I/O while a database transaction
// is open. REQUESTED is durably changed to PROCESSING first; retries reuse the
// request UUID at Xendit, so a timeout cannot create a second transfer.
func (s *RefundPayoutService) DispatchGatewayPayout(ctx context.Context, requestID string) error {
	if s.xendit == nil || !s.xendit.Configured() {
		return payment.ErrNotConfigured
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	current, err := s.payouts.LockByReferenceTx(ctx, tx, requestID)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if current.Method == "CASH" || current.Status == "PAID" || current.Status == "FAILED" || current.Status == "REVERSED" {
		_ = tx.Rollback(ctx)
		return nil
	}
	if current.Status == "REQUESTED" {
		current, err = s.payouts.TransitionTx(ctx, tx, current.ID, "PROCESSING", "xendit-worker", "", "")
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	var result *payment.Payout
	if current.ProviderPayoutID != "" {
		result, err = s.xendit.FetchPayout(ctx, current.ProviderPayoutID)
	} else {
		account, openErr := s.payouts.DestinationAccount(current)
		if openErr != nil {
			return openErr
		}
		given, surname := splitRecipientName(current.PilgrimName)
		routingType := "SWIFT"
		if current.Method == "EWALLET" {
			routingType = "WALLET"
		}
		result, err = s.xendit.CreatePayout(ctx, payment.CreatePayoutRequest{ReferenceID: current.ID, IdempotencyKey: "refund-payout-" + current.ID, AmountIDR: current.AmountIDR, GivenName: given, Surname: surname, Phone: current.PilgrimPhone, AccountHolder: current.DestinationAccountHolder, AccountNumber: account, RoutingType: routingType, RoutingValue: current.DestinationChannel, Description: "Refund " + current.ID[:8]})
	}
	if err != nil {
		_ = s.payouts.TouchProviderAttempt(ctx, current.ID)
		return err
	}
	return s.ApplyGatewayPayout(ctx, result)
}

func splitRecipientName(name string) (string, string) {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "Customer", "Customer"
	}
	if len(parts) == 1 {
		return parts[0], parts[0]
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func (s *RefundPayoutService) ApplyGatewayPayout(ctx context.Context, result *payment.Payout) error {
	if result == nil || !isUUID(result.ReferenceID) {
		return apperror.ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	current, err := s.payouts.LockByReferenceTx(ctx, tx, result.ReferenceID)
	if err != nil {
		return err
	}
	if result.AmountIDR != 0 && result.AmountIDR != current.AmountIDR {
		return fmt.Errorf("xendit payout amount mismatch: got %d want %d", result.AmountIDR, current.AmountIDR)
	}
	if current.ProviderPayoutID != "" && result.ID != "" && current.ProviderPayoutID != result.ID {
		return errors.New("xendit payout id mismatch")
	}
	if current.Status == "REQUESTED" {
		current, err = s.payouts.TransitionTx(ctx, tx, current.ID, "PROCESSING", "xendit", "", "")
		if err != nil {
			return err
		}
	}
	status := strings.ToUpper(result.Status)
	// Xendit retries webhooks. Terminal repetitions are successful no-ops, not
	// errors that would make the provider retry forever.
	if current.Status == "REVERSED" || current.Status == "FAILED" || (current.Status == "PAID" && status == "SUCCEEDED") {
		return tx.Commit(ctx)
	}
	switch status {
	case "SUCCEEDED":
		if err := s.appendPayoutMovement(ctx, tx, current, -current.AmountIDR, "WITHDRAWAL", "refund-payout-"+current.ID, "Pencairan otomatis Xendit: "+result.ProcessorReference, "xendit"); err != nil {
			return err
		}
		_, err = s.payouts.TransitionProviderTx(ctx, tx, current.ID, "PAID", result.ID, result.Status, result.FailureCode, result.ProcessorReference)
	case "FAILED", "REJECTED", "EXPIRED", "CANCELLED":
		if current.Status == "PROCESSING" {
			_, err = s.payouts.TransitionProviderTx(ctx, tx, current.ID, "FAILED", result.ID, result.Status, result.FailureCode, result.ProcessorReference)
		}
	case "REVERSED":
		if current.Status == "PAID" {
			if err = s.appendPayoutMovement(ctx, tx, current, current.AmountIDR, "ADJUSTMENT", "refund-payout-reversed-"+current.ID, "Payout dibalik oleh Xendit", "xendit"); err == nil {
				_, err = s.payouts.TransitionProviderTx(ctx, tx, current.ID, "REVERSED", result.ID, result.Status, result.FailureCode, result.ProcessorReference)
			}
		}
	default:
		_, err = s.payouts.RecordProviderAttemptTx(ctx, tx, current.ID, result.ID, result.Status, result.FailureCode, result.ProcessorReference)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *RefundPayoutService) SettleGatewayPayout(ctx context.Context, payoutID string) error {
	if s.xendit == nil || !s.xendit.Configured() {
		return payment.ErrNotConfigured
	}
	result, err := s.xendit.FetchPayout(ctx, payoutID)
	if err != nil {
		return err
	}
	return s.ApplyGatewayPayout(ctx, result)
}

func (s *RefundPayoutService) AttachCashProof(ctx context.Context, orgID, userID, role, requestID, proofURL string) (*hajjv1.RefundPayoutRequest, error) {
	const operation = "RefundPayoutService.AttachCashProof"
	if role != "owner" && role != "admin" {
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
	current, err := s.payouts.LockByIDTx(ctx, tx, op.ID, requestID)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if current.Method != "CASH" || current.Status != "PROCESSING" {
		return nil, serviceError(operation, preconditionError("bukti hanya dapat dipasang pada pencairan tunai yang sedang diproses"))
	}
	updated, err := s.payouts.SetProofTx(ctx, tx, current.ID, proofURL)
	if err != nil {
		return nil, serviceError(operation, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serviceError(operation, err)
	}
	_ = s.audit.Write(ctx, op.ID, userID, "refund_payout_proof_uploaded", "refund_payout", current.ID, "Bukti serah terima tunai diunggah")
	return refundPayoutMessage(updated), nil
}

func validateRefundPayoutTransition(current *domain.RefundPayoutRequest, req *hajjv1.TransitionRefundPayoutRequest) (string, string, error) {
	switch req.Action {
	case hajjv1.RefundPayoutAction_REFUND_PAYOUT_ACTION_START_PROCESSING:
		if current.Status == "PROCESSING" {
			return current.Status, "refund_payout_processing", nil
		}
		if current.Status != "REQUESTED" {
			return "", "", preconditionError("hanya permintaan baru yang dapat mulai diproses")
		}
		return "PROCESSING", "refund_payout_processing", nil
	case hajjv1.RefundPayoutAction_REFUND_PAYOUT_ACTION_MARK_PAID:
		if current.Status == "PAID" {
			return current.Status, "refund_payout_paid", nil
		}
		if current.Status != "PROCESSING" {
			return "", "", preconditionError("mulai proses pencairan sebelum menandainya dibayar")
		}
		if strings.TrimSpace(req.PaymentReference) == "" {
			return "", "", preconditionError("referensi pembayaran wajib diisi")
		}
		if current.Method == "CASH" && strings.TrimSpace(current.ProofURL) == "" {
			return "", "", preconditionError("unggah bukti serah terima sebelum menandai pencairan tunai dibayar")
		}
		return "PAID", "refund_payout_paid", nil
	case hajjv1.RefundPayoutAction_REFUND_PAYOUT_ACTION_MARK_FAILED:
		if current.Status == "FAILED" {
			return current.Status, "refund_payout_failed", nil
		}
		if current.Status != "REQUESTED" && current.Status != "PROCESSING" {
			return "", "", preconditionError("pencairan yang sudah dibayar tidak dapat digagalkan")
		}
		if strings.TrimSpace(req.Note) == "" {
			return "", "", preconditionError("alasan kegagalan wajib diisi")
		}
		return "FAILED", "refund_payout_failed", nil
	default:
		return "", "", apperror.ErrValidation
	}
}
func operatorCanManageMoney(ctx context.Context) bool {
	role := middleware.OrgRoleFromCtx(ctx)
	return role == "owner" || role == "admin"
}
func refundPayoutMethodToDB(m hajjv1.RefundPayoutMethod) string {
	switch m {
	case hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_BANK_TRANSFER:
		return "BANK_TRANSFER"
	case hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_EWALLET:
		return "EWALLET"
	case hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_CASH:
		return "CASH"
	}
	return ""
}
func refundPayoutMethodFromDB(m string) hajjv1.RefundPayoutMethod {
	switch m {
	case "BANK_TRANSFER":
		return hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_BANK_TRANSFER
	case "EWALLET":
		return hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_EWALLET
	case "CASH":
		return hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_CASH
	}
	return hajjv1.RefundPayoutMethod_REFUND_PAYOUT_METHOD_UNSPECIFIED
}
func refundPayoutStatusToDB(v hajjv1.RefundPayoutStatus) string {
	switch v {
	case hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_REQUESTED:
		return "REQUESTED"
	case hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_PROCESSING:
		return "PROCESSING"
	case hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_PAID:
		return "PAID"
	case hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_FAILED:
		return "FAILED"
	case hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_REVERSED:
		return "REVERSED"
	}
	return ""
}
func refundPayoutStatusFromDB(v string) hajjv1.RefundPayoutStatus {
	switch v {
	case "REQUESTED":
		return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_REQUESTED
	case "PROCESSING":
		return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_PROCESSING
	case "PAID":
		return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_PAID
	case "FAILED":
		return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_FAILED
	case "REVERSED":
		return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_REVERSED
	}
	return hajjv1.RefundPayoutStatus_REFUND_PAYOUT_STATUS_UNSPECIFIED
}
func refundPayoutMessage(r *domain.RefundPayoutRequest) *hajjv1.RefundPayoutRequest {
	kind := hajjv1.RefundBeneficiaryKind_REFUND_BENEFICIARY_KIND_PILGRIM
	ownerID := r.PilgrimID
	if r.BeneficiaryKind == "AGENT" {
		kind = hajjv1.RefundBeneficiaryKind_REFUND_BENEFICIARY_KIND_AGENT
		ownerID = r.AgentID
	}
	m := &hajjv1.RefundPayoutRequest{Id: r.ID, PilgrimId: r.PilgrimID, PilgrimName: r.PilgrimName, PilgrimPhone: r.PilgrimPhone, AmountIdr: r.AmountIDR, Method: refundPayoutMethodFromDB(r.Method), Note: r.Note, Status: refundPayoutStatusFromDB(r.Status), ResolutionNote: r.ResolutionNote, PaymentReference: r.PaymentReference, CreatedAt: timestamppb.New(r.CreatedAt), BeneficiaryKind: kind, BeneficiaryId: ownerID, BeneficiaryName: r.PilgrimName, DestinationChannel: r.DestinationChannel, DestinationAccountHolder: r.DestinationAccountHolder, DestinationAccountLast4: r.DestinationAccountLast4, Provider: r.Provider, ProviderStatus: r.ProviderStatus, ProviderFailureCode: r.ProviderFailureCode, ProofUrl: r.ProofURL}
	if r.ProcessingAt != nil {
		m.ProcessingAt = timestamppb.New(*r.ProcessingAt)
	}
	if r.ResolvedAt != nil {
		m.ResolvedAt = timestamppb.New(*r.ResolvedAt)
	}
	return m
}
