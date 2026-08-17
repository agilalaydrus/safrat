package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
	_ = s.auditRepository.Write(ctx, op.ID, userID, "agent_payout_recorded", "agent", req.AgentId, fmt.Sprintf("Rp%d via %s", req.AmountIdr, method))
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
	credits, err := s.agentRepository.ListOrderCredits(ctx, agent.ID)
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
	transactions := make([]*hajjv1.WalletTransaction, 0, len(credits)+len(debits)+len(pendingRequests))
	for _, c := range credits {
		transactions = append(transactions, &hajjv1.WalletTransaction{Id: c.OrderID, Type: hajjv1.WalletTransactionType_WALLET_TRANSACTION_TYPE_CREDIT, AmountIdr: c.AmountIDR, Description: c.ProductName, CreatedAt: timestamppb.New(c.PaidAt)})
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
	summary, err := s.agentRepository.GetPayoutSummary(ctx, op.ID, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}
	pendingRequested, err := s.agentRepository.SumPendingRequests(ctx, agent.ID)
	if err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}
	available := summary.OutstandingIDR - pendingRequested
	if req.AmountIdr > available {
		return nil, serviceError("AgentService.RequestPayout", preconditionError("amount exceeds available balance"))
	}
	request, err := s.agentRepository.CreatePayoutRequest(ctx, op.ID, agent.ID, req.AmountIdr, req.Note)
	if err != nil {
		return nil, serviceError("AgentService.RequestPayout", err)
	}
	_ = s.auditRepository.Write(ctx, op.ID, userID, "agent_payout_requested", "agent", agent.ID, fmt.Sprintf("Rp%d", req.AmountIdr))
	return payoutRequestMessage(request, agent.Name), nil
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
		return nil, serviceError("AgentService.Delete", err)
	}
	return &hajjv1.DeleteAgentResponse{}, nil
}
func agentMessage(agent *domain.Agent) *hajjv1.Agent {
	return &hajjv1.Agent{Id: agent.ID, OperatorId: agent.OperatorID, Name: agent.Name, Phone: agent.Phone, Email: agent.Email, CommissionRate: agent.CommissionRate, Notes: agent.Notes, IsActive: agent.IsActive, PilgrimCount: agent.PilgrimCount, ReferralCode: agent.ReferralCode, Tier: agent.Tier, ReferredByAgentId: agent.ReferredByAgentID, CreatedAt: timestamppb.New(agent.CreatedAt), UpdatedAt: timestamppb.New(agent.UpdatedAt)}
}
