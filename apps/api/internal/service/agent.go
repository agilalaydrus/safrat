package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AgentService struct {
	operatorRepository *repository.OperatorRepository
	agentRepository    *repository.AgentRepository
	auditRepository    *repository.AuditRepository
	db                 *pgxpool.Pool
}

func NewAgentService(operators *repository.OperatorRepository, agents *repository.AgentRepository, audit *repository.AuditRepository, db *pgxpool.Pool) *AgentService {
	return &AgentService{operatorRepository: operators, agentRepository: agents, auditRepository: audit, db: db}
}
func (s *AgentService) Create(ctx context.Context, orgID string, req *hajjv1.CreateAgentRequest) (*hajjv1.Agent, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" || req.CommissionRate < 0 || req.CommissionRate > 100 {
		return nil, serviceError("AgentService.Create", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.Create", err)
	}
	agent, err := s.agentRepository.Create(ctx, op.ID, req.Name, req.Phone, req.Email, req.Notes, req.CommissionRate)
	if err != nil {
		return nil, serviceError("AgentService.Create", err)
	}
	return agentMessage(agent), nil
}
func (s *AgentService) Get(ctx context.Context, orgID string, req *hajjv1.GetAgentRequest) (*hajjv1.Agent, error) {
	if req == nil {
		return nil, serviceError("AgentService.Get", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.Get", err)
	}
	agent, err := s.agentRepository.GetByID(ctx, op.ID, req.AgentId)
	if err != nil {
		return nil, serviceError("AgentService.Get", err)
	}
	return agentMessage(agent), nil
}
func (s *AgentService) List(ctx context.Context, orgID string) (*hajjv1.ListAgentsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.List", err)
	}
	agents, err := s.agentRepository.ListByOperatorID(ctx, op.ID)
	if err != nil {
		return nil, serviceError("AgentService.List", err)
	}
	result := &hajjv1.ListAgentsResponse{Agents: make([]*hajjv1.Agent, 0, len(agents))}
	for _, agent := range agents {
		result.Agents = append(result.Agents, agentMessage(agent))
	}
	return result, nil
}
func (s *AgentService) ListPayouts(ctx context.Context, orgID string) (*hajjv1.ListAgentPayoutsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.ListPayouts", err)
	}
	payouts, err := s.agentRepository.ListPayouts(ctx, op.ID)
	if err != nil {
		return nil, serviceError("AgentService.ListPayouts", err)
	}
	result := &hajjv1.ListAgentPayoutsResponse{Payouts: make([]*hajjv1.AgentPayout, 0, len(payouts))}
	for _, payout := range payouts {
		result.Payouts = append(result.Payouts, agentPayoutMessage(payout))
	}
	return result, nil
}
func (s *AgentService) RecordPayout(ctx context.Context, orgID, userID string, req *hajjv1.RecordAgentPayoutRequest) (*hajjv1.AgentPayout, error) {
	if req == nil || !isUUID(req.AgentId) || req.AmountIdr <= 0 {
		return nil, serviceError("AgentService.RecordPayout", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.RecordPayout", err)
	}
	current, err := s.agentRepository.GetPayoutSummary(ctx, op.ID, req.AgentId)
	if err != nil {
		return nil, serviceError("AgentService.RecordPayout", err)
	}
	if req.AmountIdr > current.OutstandingIDR {
		return nil, serviceError("AgentService.RecordPayout", preconditionError("amount exceeds outstanding balance"))
	}
	method := payoutMethodToDB(req.Method)
	if method == "" {
		return nil, serviceError("AgentService.RecordPayout", apperror.ErrValidation)
	}
	requestID := strings.TrimSpace(req.RequestId)
	if requestID == "" {
		if err := s.agentRepository.RecordPayout(ctx, op.ID, req.AgentId, req.AmountIdr, req.Note, method, userID, ""); err != nil {
			return nil, serviceError("AgentService.RecordPayout", err)
		}
	} else {
		// Settling a leader-initiated request — the ledger insert and the
		// request's PENDING->APPROVED transition must commit together, or a
		// crash between the two either loses the money movement or leaves
		// the request stuck open after it's already been paid.
		pending, err := s.agentRepository.GetPayoutRequest(ctx, op.ID, requestID)
		if err != nil {
			return nil, serviceError("AgentService.RecordPayout", err)
		}
		if pending.AgentID != req.AgentId || pending.Status != "PENDING" {
			return nil, serviceError("AgentService.RecordPayout", preconditionError("payout request is not a pending request for this agent"))
		}
		tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return nil, serviceError("AgentService.RecordPayout", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := s.agentRepository.RecordPayoutTx(ctx, tx, op.ID, req.AgentId, req.AmountIdr, req.Note, method, userID, requestID); err != nil {
			return nil, serviceError("AgentService.RecordPayout", err)
		}
		if err := s.agentRepository.ApprovePayoutRequestTx(ctx, tx, op.ID, requestID, userID); err != nil {
			return nil, serviceError("AgentService.RecordPayout", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, serviceError("AgentService.RecordPayout", err)
		}
	}
	_ = s.auditRepository.Write(ctx, op.ID, userID, "agent_payout_recorded", "agent", req.AgentId, fmt.Sprintf("%s via %s", rupiah(req.AmountIdr), method))
	updated, err := s.agentRepository.GetPayoutSummary(ctx, op.ID, req.AgentId)
	if err != nil {
		return nil, serviceError("AgentService.RecordPayout", err)
	}
	return agentPayoutMessage(updated), nil
}

// GetMyWallet resolves the caller's own agent record (every Group Leader is
// also an agent) and returns their balance plus a merged, chronological
// transaction history — commission credits from PAID orders, disbursement
// debits from the agent_payouts ledger, and their own pending withdrawal
// requests (shown so they can see that money is already spoken for).
func (s *AgentService) GetMyWallet(ctx context.Context, orgID, userID string) (*hajjv1.AgentWallet, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.GetMyWallet", err)
	}
	agent, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("AgentService.GetMyWallet", err)
	}
	summary, err := s.agentRepository.GetPayoutSummary(ctx, op.ID, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.GetMyWallet", err)
	}
	pendingRequested, err := s.agentRepository.SumPendingRequests(ctx, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.GetMyWallet", err)
	}
	entries, err := s.agentRepository.ListCommissionEntries(ctx, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.GetMyWallet", err)
	}
	debits, err := s.agentRepository.ListPayoutHistory(ctx, op.ID, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.GetMyWallet", err)
	}
	pendingRequests, err := s.agentRepository.ListPayoutRequests(ctx, op.ID, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.GetMyWallet", err)
	}
	transactions := make([]*hajjv1.WalletTransaction, 0, len(entries)+len(debits)+len(pendingRequests))
	for _, e := range entries {
		// A reversal is stored as a negative amount, but it reads as a
		// withdrawal of commission rather than a negative earning — the type
		// carries the direction, so the amount is shown as a magnitude.
		amount := e.AmountIDR
		if amount < 0 {
			amount = -amount
		}
		transactions = append(transactions, &hajjv1.WalletTransaction{
			Id: e.ID, Type: commissionEntryType(e.Kind), AmountIdr: amount,
			Description: commissionEntryDescription(e), CreatedAt: timestamppb.New(e.CreatedAt),
		})
	}
	for _, d := range debits {
		transactions = append(transactions, &hajjv1.WalletTransaction{Id: d.ID, Type: hajjv1.WalletTransactionType_WALLET_TRANSACTION_TYPE_DEBIT, AmountIdr: d.AmountIDR, Description: payoutMethodLabel(d.Method), CreatedAt: timestamppb.New(d.CreatedAt)})
	}
	for _, r := range pendingRequests {
		transactions = append(transactions, &hajjv1.WalletTransaction{Id: r.ID, Type: hajjv1.WalletTransactionType_WALLET_TRANSACTION_TYPE_PENDING_REQUEST, AmountIdr: r.AmountIDR, Description: "Menunggu persetujuan", CreatedAt: timestamppb.New(r.RequestedAt)})
	}
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].CreatedAt.AsTime().After(transactions[j].CreatedAt.AsTime())
	})
	return &hajjv1.AgentWallet{
		AgentId: agent.ID, AgentName: agent.Name,
		TotalEarnedIdr: summary.TotalCommissionIDR, TotalWithdrawnIdr: summary.TotalDisbursedIDR,
		BalanceIdr: summary.OutstandingIDR, PendingRequestedIdr: pendingRequested,
		AvailableIdr: summary.OutstandingIDR - pendingRequested, Transactions: transactions,
		PendingCommissionIdr: summary.PendingCommissionIDR,
	}, nil
}

func (s *AgentService) RequestPayout(ctx context.Context, orgID, userID string, req *hajjv1.RequestAgentPayoutRequest) (*hajjv1.PayoutRequest, error) {
	if req == nil || req.AmountIdr <= 0 {
		return nil, serviceError("AgentService.RequestPayout", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}
	agent, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}
	// Serialised per agent. Reading the balance and inserting the request are
	// two statements, so two concurrent calls — a double-clicked button, or a
	// retried request — would both see the same available figure and both pass
	// the check, letting an agent request more money than they are owed. The
	// lock is held for the transaction, so the second caller reads a balance
	// that already accounts for the first.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, agent.ID); err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}

	summary, err := s.agentRepository.GetPayoutSummaryTx(ctx, tx, op.ID, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}
	pendingRequested, err := s.agentRepository.SumPendingRequestsTx(ctx, tx, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}
	available := summary.OutstandingIDR - pendingRequested
	if req.AmountIdr > available {
		return nil, serviceError("AgentService.RequestPayout", preconditionError("amount exceeds available balance"))
	}
	request, err := s.agentRepository.CreatePayoutRequestTx(ctx, tx, op.ID, agent.ID, req.AmountIdr, req.Note)
	if err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}
	_ = s.auditRepository.Write(ctx, op.ID, userID, "agent_payout_requested", "agent", agent.ID, rupiah(req.AmountIdr))
	return payoutRequestMessage(request, agent.Name), nil
}

// ListMyPilgrims resolves the caller's own agent record (same pattern as
// GetMyWallet) and returns every pilgrim they've referred.
func (s *AgentService) ListMyPilgrims(ctx context.Context, orgID, userID string) (*hajjv1.ListMyPilgrimsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.ListMyPilgrims", err)
	}
	agent, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("AgentService.ListMyPilgrims", err)
	}
	rows, err := s.agentRepository.ListMyPilgrims(ctx, op.ID, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.ListMyPilgrims", err)
	}
	result := &hajjv1.ListMyPilgrimsResponse{Pilgrims: make([]*hajjv1.AgentPilgrim, 0, len(rows))}
	for _, row := range rows {
		msg := &hajjv1.AgentPilgrim{
			Id: row.ID, FullName: row.FullName, PassportNumber: row.PassportNumber, Gender: row.Gender,
			PaymentStatus: row.PaymentStatus, DocsComplete: row.DocsComplete, PilgrimStatus: row.PilgrimStatus, SeasonName: row.SeasonName, SeasonId: row.SeasonID,
		}
		if row.DepartureDate != nil {
			msg.DepartureDate = timestamppb.New(*row.DepartureDate)
		}
		result.Pilgrims = append(result.Pilgrims, msg)
	}
	return result, nil
}

func (s *AgentService) ListPayoutRequests(ctx context.Context, orgID string, req *hajjv1.ListPayoutRequestsRequest) (*hajjv1.ListPayoutRequestsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.ListPayoutRequests", err)
	}
	agentID := ""
	if req != nil {
		agentID = req.AgentId
	}
	requests, err := s.agentRepository.ListPayoutRequests(ctx, op.ID, agentID)
	if err != nil {
		return nil, serviceError("AgentService.ListPayoutRequests", err)
	}
	result := &hajjv1.ListPayoutRequestsResponse{Requests: make([]*hajjv1.PayoutRequest, 0, len(requests))}
	for _, r := range requests {
		result.Requests = append(result.Requests, payoutRequestMessage(r, r.AgentName))
	}
	return result, nil
}

func (s *AgentService) RejectPayoutRequest(ctx context.Context, orgID, userID string, req *hajjv1.RejectPayoutRequestRequest) (*hajjv1.PayoutRequest, error) {
	if req == nil || !isUUID(req.RequestId) || strings.TrimSpace(req.Note) == "" {
		return nil, serviceError("AgentService.RejectPayoutRequest", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.RejectPayoutRequest", err)
	}
	request, err := s.agentRepository.RejectPayoutRequest(ctx, op.ID, req.RequestId, userID, req.Note)
	if err != nil {
		return nil, serviceError("AgentService.RejectPayoutRequest", err)
	}
	_ = s.auditRepository.Write(ctx, op.ID, userID, "agent_payout_rejected", "agent", request.AgentID, req.Note)
	return payoutRequestMessage(request, ""), nil
}

func payoutRequestMessage(r *domain.PayoutRequest, agentName string) *hajjv1.PayoutRequest {
	name := r.AgentName
	if name == "" {
		name = agentName
	}
	msg := &hajjv1.PayoutRequest{
		Id: r.ID, AgentId: r.AgentID, AgentName: name, AmountIdr: r.AmountIDR, Note: r.Note,
		Status: payoutRequestStatusFromDB(r.Status), RequestedAt: timestamppb.New(r.RequestedAt), ResolutionNote: r.ResolutionNote,
	}
	if r.ResolvedAt != nil {
		msg.ResolvedAt = timestamppb.New(*r.ResolvedAt)
	}
	return msg
}

func payoutRequestStatusFromDB(s string) hajjv1.PayoutRequestStatus {
	switch s {
	case "PENDING":
		return hajjv1.PayoutRequestStatus_PAYOUT_REQUEST_STATUS_PENDING
	case "APPROVED":
		return hajjv1.PayoutRequestStatus_PAYOUT_REQUEST_STATUS_APPROVED
	case "REJECTED":
		return hajjv1.PayoutRequestStatus_PAYOUT_REQUEST_STATUS_REJECTED
	default:
		return hajjv1.PayoutRequestStatus_PAYOUT_REQUEST_STATUS_UNSPECIFIED
	}
}

func payoutMethodLabel(m string) string {
	switch m {
	case "TRANSFER":
		return "Transfer Bank"
	case "CASH":
		return "Tunai"
	case "EWALLET":
		return "E-Wallet"
	default:
		return m
	}
}
func (s *AgentService) ListPayoutHistory(ctx context.Context, orgID string, req *hajjv1.ListAgentPayoutHistoryRequest) (*hajjv1.ListAgentPayoutHistoryResponse, error) {
	if req == nil || !isUUID(req.AgentId) {
		return nil, serviceError("AgentService.ListPayoutHistory", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.ListPayoutHistory", err)
	}
	entries, err := s.agentRepository.ListPayoutHistory(ctx, op.ID, req.AgentId)
	if err != nil {
		return nil, serviceError("AgentService.ListPayoutHistory", err)
	}
	result := &hajjv1.ListAgentPayoutHistoryResponse{Entries: make([]*hajjv1.AgentPayoutEntry, 0, len(entries))}
	for _, entry := range entries {
		result.Entries = append(result.Entries, &hajjv1.AgentPayoutEntry{
			Id:         entry.ID,
			AmountIdr:  entry.AmountIDR,
			Note:       entry.Note,
			Method:     payoutMethodFromDB(entry.Method),
			PaidByName: entry.PaidByName,
			CreatedAt:  timestamppb.New(entry.CreatedAt),
		})
	}
	return result, nil
}
func payoutMethodToDB(m hajjv1.PayoutMethod) string {
	switch m {
	case hajjv1.PayoutMethod_PAYOUT_METHOD_TRANSFER:
		return "TRANSFER"
	case hajjv1.PayoutMethod_PAYOUT_METHOD_CASH:
		return "CASH"
	case hajjv1.PayoutMethod_PAYOUT_METHOD_EWALLET:
		return "EWALLET"
	default:
		return ""
	}
}
func payoutMethodFromDB(m string) hajjv1.PayoutMethod {
	switch m {
	case "TRANSFER":
		return hajjv1.PayoutMethod_PAYOUT_METHOD_TRANSFER
	case "CASH":
		return hajjv1.PayoutMethod_PAYOUT_METHOD_CASH
	case "EWALLET":
		return hajjv1.PayoutMethod_PAYOUT_METHOD_EWALLET
	default:
		return hajjv1.PayoutMethod_PAYOUT_METHOD_UNSPECIFIED
	}
}
func agentPayoutMessage(payout *domain.AgentPayout) *hajjv1.AgentPayout {
	return &hajjv1.AgentPayout{
		AgentId:            payout.AgentID,
		AgentName:          payout.AgentName,
		TotalCommissionIdr: payout.TotalCommissionIDR,
		PaidOrderCount:     payout.PaidOrderCount,
		TotalDisbursedIdr:  payout.TotalDisbursedIDR,
		OutstandingIdr:     payout.OutstandingIDR,
	}
}
func (s *AgentService) Update(ctx context.Context, orgID string, req *hajjv1.UpdateAgentRequest) (*hajjv1.Agent, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" || req.CommissionRate < 0 || req.CommissionRate > 100 {
		return nil, serviceError("AgentService.Update", apperror.ErrValidation)
	}
	if _, err := uuid.Parse(req.AgentId); err != nil {
		return nil, serviceError("AgentService.Update", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.Update", err)
	}
	agent, err := s.agentRepository.Update(ctx, op.ID, req.AgentId, req.Name, req.Phone, req.Email, req.Notes, req.CommissionRate, req.IsActive)
	if err != nil {
		return nil, serviceError("AgentService.Update", err)
	}
	return agentMessage(agent), nil
}

// ApplyAsAgent is the public application path — there is no authenticated
// operator identity here, so operator_id comes from the request itself.
func (s *AgentService) ApplyAsAgent(ctx context.Context, req *hajjv1.ApplyAsAgentRequest) (*hajjv1.Agent, error) {
	if req == nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.OperatorId) == "" {
		return nil, serviceError("AgentService.ApplyAsAgent", apperror.ErrValidation)
	}
	if _, err := s.operatorRepository.GetByID(ctx, req.OperatorId); err != nil {
		return nil, serviceError("AgentService.ApplyAsAgent", err)
	}
	referredByAgentID := ""
	if code := strings.TrimSpace(req.ReferredByCode); code != "" {
		referrer, err := s.agentRepository.GetByReferralCode(ctx, req.OperatorId, code)
		if err == nil {
			referredByAgentID = referrer.ID
		}
	}
	agent, err := s.agentRepository.CreateApplication(ctx, req.OperatorId, req.Name, req.Phone, req.Email, referredByAgentID)
	if err != nil {
		return nil, serviceError("AgentService.ApplyAsAgent", err)
	}
	return agentMessage(agent), nil
}

// RecalculateTiers is invoked by the tier-recalculation worker (cmd/worker), not by
// any RPC. Payout itself is a separate concern — see ListPayouts, which sums
// each order's already-frozen agent_commission_idr rather than projecting
// off commissionRate, so a later rate change never rewrites what's owed for
// past orders.
func (s *AgentService) RecalculateTiers(ctx context.Context, operatorID string) error {
	rows, err := s.agentRepository.ListActiveForTiering(ctx, operatorID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		tier := tierForPilgrimCount(row.PilgrimCount)
		if tier == row.Tier {
			continue
		}
		agentID := uuid.UUID(row.ID.Bytes).String()
		if err := s.agentRepository.UpdateTier(ctx, operatorID, agentID, tier); err != nil {
			return err
		}
	}
	return nil
}

func tierForPilgrimCount(count int32) string {
	switch {
	case count >= 30:
		return "GOLD"
	case count >= 10:
		return "SILVER"
	default:
		return "BRONZE"
	}
}

func (s *AgentService) Delete(ctx context.Context, orgID string, req *hajjv1.DeleteAgentRequest) (*hajjv1.DeleteAgentResponse, error) {
	if req == nil {
		return nil, serviceError("AgentService.Delete", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.Delete", err)
	}
	if err := s.agentRepository.Delete(ctx, op.ID, req.AgentId); err != nil {
		// An agent who has earned commission cannot be removed — deleting them
		// would erase the record of money they were owed. Deactivating is the
		// supported path, and the operator needs to be told that rather than
		// shown a foreign key error.
		if repository.IsForeignKeyViolation(err, "agent_commission_entries_agent_id_fkey") {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("agen sudah memiliki riwayat komisi dan tidak dapat dihapus; nonaktifkan agen ini agar riwayatnya tetap tersimpan"))
		}
		return nil, serviceError("AgentService.Delete", err)
	}
	return &hajjv1.DeleteAgentResponse{}, nil
}
func agentMessage(agent *domain.Agent) *hajjv1.Agent {
	result := &hajjv1.Agent{
		Id: agent.ID, OperatorId: agent.OperatorID, Name: agent.Name, Phone: agent.Phone, Email: agent.Email, CommissionRate: agent.CommissionRate,
		Notes: agent.Notes, IsActive: agent.IsActive, PilgrimCount: agent.PilgrimCount, ReferralCode: agent.ReferralCode, Tier: agent.Tier,
		ReferredByAgentId: agent.ReferredByAgentID, CreatedAt: timestamppb.New(agent.CreatedAt), UpdatedAt: timestamppb.New(agent.UpdatedAt),
		Nik: agent.NIK, Npwp: agent.NPWP, Address: agent.Address, PassportNumber: agent.PassportNumber,
		BankName: agent.BankName, BankAccountNumber: agent.BankAccountNumber, BankAccountHolder: agent.BankAccountHolder,
		KycStatus: agent.KYCStatus, KycSource: agent.KYCSource, KycVerifiedBy: agent.KYCVerifiedBy, KycRejectionReason: agent.KYCRejectionReason,
	}
	if agent.DateOfBirth != nil {
		result.DateOfBirth = timestamppb.New(*agent.DateOfBirth)
	}
	if agent.PassportExpiryDate != nil {
		result.PassportExpiryDate = timestamppb.New(*agent.PassportExpiryDate)
	}
	if agent.KYCVerifiedAt != nil {
		result.KycVerifiedAt = timestamppb.New(*agent.KYCVerifiedAt)
	}
	return result
}

func agentKycInputFromRequest(nik, npwp, address string, dateOfBirth *timestamppb.Timestamp, passportNumber string, passportExpiryDate *timestamppb.Timestamp, bankName, bankAccountNumber, bankAccountHolder string) domain.AgentKYCInput {
	input := domain.AgentKYCInput{NIK: nik, NPWP: npwp, Address: address, PassportNumber: passportNumber, BankName: bankName, BankAccountNumber: bankAccountNumber, BankAccountHolder: bankAccountHolder}
	if dateOfBirth != nil {
		t := dateOfBirth.AsTime()
		input.DateOfBirth = &t
	}
	if passportExpiryDate != nil {
		t := passportExpiryDate.AsTime()
		input.PassportExpiryDate = &t
	}
	return input
}

func (s *AgentService) UpdateKyc(ctx context.Context, authenticatedOrgID string, req *hajjv1.UpdateAgentKycRequest) (*hajjv1.Agent, error) {
	if req == nil || req.AgentId == "" {
		return nil, serviceError("AgentService.UpdateKyc", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("AgentService.UpdateKyc", err)
	}
	input := agentKycInputFromRequest(req.Nik, req.Npwp, req.Address, req.DateOfBirth, req.PassportNumber, req.PassportExpiryDate, req.BankName, req.BankAccountNumber, req.BankAccountHolder)
	agent, err := s.agentRepository.UpdateKYC(ctx, op.ID, req.AgentId, input, "ADMIN")
	if err != nil {
		return nil, serviceError("AgentService.UpdateKyc", err)
	}
	return agentMessage(agent), nil
}

func (s *AgentService) VerifyKyc(ctx context.Context, authenticatedOrgID, verifiedBy string, req *hajjv1.VerifyAgentKycRequest) (*hajjv1.Agent, error) {
	if req == nil || req.AgentId == "" {
		return nil, serviceError("AgentService.VerifyKyc", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("AgentService.VerifyKyc", err)
	}
	agent, err := s.agentRepository.VerifyKYC(ctx, op.ID, req.AgentId, verifiedBy, req.Approve, req.RejectionReason)
	if err != nil {
		return nil, serviceError("AgentService.VerifyKyc", err)
	}
	return agentMessage(agent), nil
}

// SubmitMyKyc and GetMyKyc resolve "which agent" from the caller's own
// Better Auth identity, same as GetMyWallet — never trust an agent_id from
// the request. Works for both a real Agent and a Muttawwif (every leader
// has an agent row via EnsureAgentForLeader).
func (s *AgentService) SubmitMyKyc(ctx context.Context, orgID, userID string, req *hajjv1.SubmitMyAgentKycRequest) (*hajjv1.Agent, error) {
	if req == nil {
		return nil, serviceError("AgentService.SubmitMyKyc", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.SubmitMyKyc", err)
	}
	self, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("AgentService.SubmitMyKyc", err)
	}
	input := agentKycInputFromRequest(req.Nik, req.Npwp, req.Address, req.DateOfBirth, req.PassportNumber, req.PassportExpiryDate, req.BankName, req.BankAccountNumber, req.BankAccountHolder)
	agent, err := s.agentRepository.UpdateKYC(ctx, op.ID, self.ID, input, "SELF")
	if err != nil {
		return nil, serviceError("AgentService.SubmitMyKyc", err)
	}
	return agentMessage(agent), nil
}

func (s *AgentService) GetMyKyc(ctx context.Context, orgID, userID string) (*hajjv1.Agent, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.GetMyKyc", err)
	}
	self, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("AgentService.GetMyKyc", err)
	}
	return agentMessage(self), nil
}

var validAgentDocTypes = map[string]bool{"KTP": true, "PASSPORT": true, "SELFIE": true, "NPWP": true, "BANK_BOOK": true, "OTHER": true}

// CreateDocument is called from the plain HTTP multipart upload endpoint
// (see main.go) by an admin, uploaded_by "operator".
func (s *AgentService) CreateDocument(ctx context.Context, authenticatedOrgID, agentID, docType, fileURL, fileName string) (*domain.AgentDocument, error) {
	if agentID == "" || !validAgentDocTypes[docType] || fileURL == "" || fileName == "" {
		return nil, serviceError("AgentService.CreateDocument", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("AgentService.CreateDocument", err)
	}
	document, err := s.agentRepository.CreateDocument(ctx, op.ID, agentID, docType, fileURL, fileName, "operator")
	if err != nil {
		return nil, serviceError("AgentService.CreateDocument", err)
	}
	return document, nil
}

// CreateDocumentSelf is the Agent/Muttawwif self-upload counterpart —
// resolves "which agent" from the caller's own identity, same as
// GetMyWallet, uploaded_by "self".
func (s *AgentService) CreateDocumentSelf(ctx context.Context, orgID, userID, docType, fileURL, fileName string) (*domain.AgentDocument, error) {
	if !validAgentDocTypes[docType] || fileURL == "" || fileName == "" {
		return nil, serviceError("AgentService.CreateDocumentSelf", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.CreateDocumentSelf", err)
	}
	self, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("AgentService.CreateDocumentSelf", err)
	}
	document, err := s.agentRepository.CreateDocument(ctx, op.ID, self.ID, docType, fileURL, fileName, "self")
	if err != nil {
		return nil, serviceError("AgentService.CreateDocumentSelf", err)
	}
	return document, nil
}

func agentDocumentMessage(doc *domain.AgentDocument) *hajjv1.AgentDocument {
	return &hajjv1.AgentDocument{Id: doc.ID, AgentId: doc.AgentID, DocType: doc.DocType, FileUrl: doc.FileURL, FileName: doc.FileName, UploadedBy: doc.UploadedBy, CreatedAt: timestamppb.New(doc.CreatedAt)}
}

func (s *AgentService) ListDocuments(ctx context.Context, authenticatedOrgID string, req *hajjv1.ListAgentDocumentsRequest) (*hajjv1.ListAgentDocumentsResponse, error) {
	if req == nil || req.AgentId == "" {
		return nil, serviceError("AgentService.ListDocuments", apperror.ErrValidation)
	}
	if _, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID); err != nil {
		return nil, serviceError("AgentService.ListDocuments", err)
	}
	documents, err := s.agentRepository.ListDocuments(ctx, req.AgentId)
	if err != nil {
		return nil, serviceError("AgentService.ListDocuments", err)
	}
	result := &hajjv1.ListAgentDocumentsResponse{Documents: make([]*hajjv1.AgentDocument, 0, len(documents))}
	for _, doc := range documents {
		result.Documents = append(result.Documents, agentDocumentMessage(doc))
	}
	return result, nil
}

// ListMyDocuments resolves "which agent" from the caller's own identity,
// same as GetMyWallet.
func (s *AgentService) ListMyDocuments(ctx context.Context, orgID, userID string) (*hajjv1.ListAgentDocumentsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.ListMyDocuments", err)
	}
	self, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("AgentService.ListMyDocuments", err)
	}
	documents, err := s.agentRepository.ListDocuments(ctx, self.ID)
	if err != nil {
		return nil, serviceError("AgentService.ListMyDocuments", err)
	}
	result := &hajjv1.ListAgentDocumentsResponse{Documents: make([]*hajjv1.AgentDocument, 0, len(documents))}
	for _, doc := range documents {
		result.Documents = append(result.Documents, agentDocumentMessage(doc))
	}
	return result, nil
}

func commissionEntryType(kind string) hajjv1.WalletTransactionType {
	switch kind {
	case "EARNED":
		return hajjv1.WalletTransactionType_WALLET_TRANSACTION_TYPE_CREDIT
	case "REVERSED":
		return hajjv1.WalletTransactionType_WALLET_TRANSACTION_TYPE_REVERSAL
	default:
		return hajjv1.WalletTransactionType_WALLET_TRANSACTION_TYPE_ADJUSTMENT
	}
}

// The product name alone would leave a reversal looking identical to the
// earning it cancels, so a reversal says what it is and why.
func commissionEntryDescription(entry *domain.CommissionEntry) string {
	switch {
	case entry.Kind == "REVERSED" && entry.ProductName != "":
		return "Komisi ditarik — " + entry.ProductName
	case entry.Kind == "REVERSED":
		return "Komisi ditarik kembali"
	case entry.ProductName != "":
		return entry.ProductName
	case entry.Note != "":
		return entry.Note
	default:
		return "Komisi"
	}
}

// ListMyReferredTransactions is the money view of an agent's referral list:
// what each jamaah they referred actually transacted, and what survived after
// refunds. Self-scoped from the caller's own identity — an agent id in the
// request would be a way to read someone else's book.
func (s *AgentService) ListMyReferredTransactions(ctx context.Context, orgID, userID string) (*hajjv1.ListMyReferredTransactionsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("AgentService.ListMyReferredTransactions", err)
	}
	agent, err := s.agentRepository.GetByLinkedUser(ctx, op.ID, userID)
	if err != nil {
		return nil, serviceError("AgentService.ListMyReferredTransactions", err)
	}
	recaps, err := s.agentRepository.ListReferredCustomerRecap(ctx, op.ID, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.ListMyReferredTransactions", err)
	}
	result := &hajjv1.ListMyReferredTransactionsResponse{Customers: make([]*hajjv1.ReferredCustomerRecap, 0, len(recaps))}
	for _, recap := range recaps {
		result.TotalPaidIdr += recap.TotalPaidIDR
		result.TotalRefundedIdr += recap.RefundedIDR
		result.TotalCommissionIdr += recap.CommissionIDR
		result.Customers = append(result.Customers, &hajjv1.ReferredCustomerRecap{
			PilgrimId: recap.PilgrimID, PilgrimName: recap.PilgrimName,
			OrderCount: recap.OrderCount, RefundedOrderCount: recap.RefundedOrderCount,
			TotalPaidIdr: recap.TotalPaidIDR, RefundedIdr: recap.RefundedIDR,
			CommissionIdr:     recap.CommissionIDR,
			LastTransactionAt: timestamppb.New(recap.LastTransactionAt),
		})
	}
	return result, nil
}
