package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type AgentRepository struct{ queries *db.Queries }

func NewAgentRepository(queries *db.Queries) *AgentRepository {
	return &AgentRepository{queries: queries}
}

func (r *AgentRepository) Create(ctx context.Context, operatorID, name, phone, email, notes string, commissionRate float64) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agent, err := r.queries.CreateAgent(ctx, db.CreateAgentParams{OperatorID: opUUID, Name: name, Phone: phone, Email: email, CommissionRate: commissionRate, Notes: notes, IsActive: true})
	if err != nil {
		return nil, err
	}
	return toAgent(agent, 0), nil
}

// EnsureAgentForLeader creates an agent record for this Better Auth user if
// one doesn't already exist for them at this operator — idempotent, so
// reassigning the same person as a group's leader again (or as a second
// group's leader) never creates a duplicate. Pre-fills name/email from
// their real account instead of a blank form; commission_rate/tier/
// referral_code all take the same column defaults CreateAgent relies on.
// UpdateKYC is used both by an admin editing an agent/Muttawwif's KYC
// fields and by the caller submitting their own (kycSource distinguishes
// which) — resets status to PENDING_REVIEW and clears prior verification.
func (r *AgentRepository) UpdateKYC(ctx context.Context, operatorID, agentID string, input domain.AgentKYCInput, kycSource string) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.UpdateAgentKyc(ctx, db.UpdateAgentKycParams{
		ID: agentUUID, OperatorID: opUUID, Nik: input.NIK, Npwp: input.NPWP, Address: input.Address,
		DateOfBirth: pgDate(input.DateOfBirth), PassportNumber: input.PassportNumber, PassportExpiryDate: pgDate(input.PassportExpiryDate),
		BankName: input.BankName, BankAccountNumber: input.BankAccountNumber, BankAccountHolder: input.BankAccountHolder,
		KycStatus: "PENDING_REVIEW", KycSource: kycSource,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toAgent(row, 0), nil
}

func (r *AgentRepository) VerifyKYC(ctx context.Context, operatorID, agentID, verifiedBy string, approve bool, rejectionReason string) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	status := "VERIFIED"
	if !approve {
		status = "REJECTED"
	}
	row, err := r.queries.VerifyAgentKyc(ctx, db.VerifyAgentKycParams{
		ID: agentUUID, OperatorID: opUUID, KycStatus: status, KycVerifiedBy: verifiedBy, KycRejectionReason: rejectionReason,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toAgent(row, 0), nil
}

func (r *AgentRepository) CreateDocument(ctx context.Context, operatorID, agentID, docType, fileURL, fileName, uploadedBy string) (*domain.AgentDocument, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.CreateAgentDocument(ctx, db.CreateAgentDocumentParams{
		AgentID: agentUUID, OperatorID: opUUID, DocType: docType, FileUrl: fileURL, FileName: fileName, UploadedBy: uploadedBy,
	})
	if err != nil {
		return nil, databaseError(err)
	}
	return toAgentDocument(row), nil
}

func (r *AgentRepository) ListDocuments(ctx context.Context, agentID string) ([]*domain.AgentDocument, error) {
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListAgentDocuments(ctx, agentUUID)
	if err != nil {
		return nil, databaseError(err)
	}
	result := make([]*domain.AgentDocument, 0, len(rows))
	for _, row := range rows {
		result = append(result, toAgentDocument(row))
	}
	return result, nil
}

func toAgentDocument(value db.AgentDocument) *domain.AgentDocument {
	return &domain.AgentDocument{
		ID: uuid.UUID(value.ID.Bytes).String(), AgentID: uuid.UUID(value.AgentID.Bytes).String(),
		DocType: value.DocType, FileURL: value.FileUrl, FileName: value.FileName, UploadedBy: value.UploadedBy, CreatedAt: value.CreatedAt.Time,
	}
}

func (r *AgentRepository) EnsureAgentForLeader(ctx context.Context, operatorID, userID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	userIDText := pgtype.Text{String: userID, Valid: true}
	_, err = r.queries.GetAgentByLinkedUser(ctx, db.GetAgentByLinkedUserParams{OperatorID: opUUID, LinkedUserID: userIDText})
	if err == nil {
		return nil // already an agent
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	user, err := r.queries.GetUserForAgent(ctx, userID)
	if err != nil {
		return err
	}
	_, err = r.queries.CreateAgentForLeader(ctx, db.CreateAgentForLeaderParams{OperatorID: opUUID, Name: user.Name, Phone: "", Email: user.Email, LinkedUserID: userIDText})
	return err
}

func (r *AgentRepository) GetByID(ctx context.Context, operatorID, agentID string) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	agent, err := r.queries.GetAgent(ctx, db.GetAgentParams{ID: agentUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	return toAgent(agent, 0), nil
}

func (r *AgentRepository) ListByOperatorID(ctx context.Context, operatorID string) ([]*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListAgentsWithPilgrimCount(ctx, opUUID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Agent, 0, len(rows))
	for _, row := range rows {
		agent := db.Agent{ID: row.ID, OperatorID: row.OperatorID, Name: row.Name, Phone: row.Phone, Email: row.Email, CommissionRate: row.CommissionRate, Notes: row.Notes, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
		result = append(result, toAgent(agent, row.PilgrimCount))
	}
	return result, nil
}

func (r *AgentRepository) ListPayouts(ctx context.Context, operatorID string) ([]*domain.AgentPayout, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListAgentPayouts(ctx, opUUID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.AgentPayout, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.AgentPayout{
			AgentID:            uuid.UUID(row.AgentID.Bytes).String(),
			AgentName:          row.AgentName,
			TotalCommissionIDR: row.TotalCommissionIdr,
			PaidOrderCount:     row.PaidOrderCount,
			TotalDisbursedIDR:  row.TotalDisbursedIdr,
			OutstandingIDR:     row.TotalCommissionIdr - row.TotalDisbursedIdr,
		})
	}
	return result, nil
}

func (r *AgentRepository) GetPayoutSummary(ctx context.Context, operatorID, agentID string) (*domain.AgentPayout, error) {
	return r.payoutSummary(ctx, r.queries, operatorID, agentID)
}

func (r *AgentRepository) payoutSummary(ctx context.Context, q *db.Queries, operatorID, agentID string) (*domain.AgentPayout, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	row, err := q.GetAgentPayoutSummary(ctx, db.GetAgentPayoutSummaryParams{OperatorID: opUUID, ID: agentUUID})
	if err != nil {
		return nil, err
	}
	return &domain.AgentPayout{
		AgentID:            uuid.UUID(row.AgentID.Bytes).String(),
		AgentName:          row.AgentName,
		TotalCommissionIDR: row.TotalCommissionIdr,
		PaidOrderCount:     row.PaidOrderCount,
		TotalDisbursedIDR:  row.TotalDisbursedIdr,
		OutstandingIDR:     row.TotalCommissionIdr - row.TotalDisbursedIdr,
	}, nil
}

// RecordPayout appends a disbursement to the ledger. Callers are expected to
// have already validated amount against the current outstanding balance
// (see AgentService.RecordPayout) — this just persists the entry.
func (r *AgentRepository) RecordPayout(ctx context.Context, operatorID, agentID string, amountIDR int64, note, method, paidByUserID, requestID string) error {
	return r.recordPayout(ctx, r.queries, operatorID, agentID, amountIDR, note, method, paidByUserID, requestID)
}

// RecordPayoutTx is the same write, scoped to a caller-managed transaction —
// used when settling a withdrawal request, where the ledger insert and the
// request's APPROVED transition must commit together or not at all.
func (r *AgentRepository) RecordPayoutTx(ctx context.Context, tx pgx.Tx, operatorID, agentID string, amountIDR int64, note, method, paidByUserID, requestID string) error {
	return r.recordPayout(ctx, r.queries.WithTx(tx), operatorID, agentID, amountIDR, note, method, paidByUserID, requestID)
}

func (r *AgentRepository) recordPayout(ctx context.Context, q *db.Queries, operatorID, agentID string, amountIDR int64, note, method, paidByUserID, requestID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return err
	}
	_, err = q.RecordAgentPayout(ctx, db.RecordAgentPayoutParams{
		OperatorID:   opUUID,
		AgentID:      agentUUID,
		AmountIdr:    amountIDR,
		Note:         note,
		Method:       method,
		PaidByUserID: paidByUserID,
		Column7:      requestID,
	})
	return err
}

// GetByLinkedUser resolves the agent record for a Better Auth identity —
// every Group Leader is also an agent (EnsureAgentForLeader), so this is how
// self-service wallet RPCs figure out "which agent is the caller" without
// trusting an agent_id from the request.
func (r *AgentRepository) GetByLinkedUser(ctx context.Context, operatorID, userID string) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agent, err := r.queries.GetAgentByLinkedUser(ctx, db.GetAgentByLinkedUserParams{OperatorID: opUUID, LinkedUserID: pgtype.Text{String: userID, Valid: true}})
	if err != nil {
		return nil, err
	}
	return toAgent(agent, 0), nil
}

// ListMyPilgrims returns every pilgrim referred by this agent, across all
// seasons — the Tour Leader portal's "Jamaah Saya" tab.
func (r *AgentRepository) ListMyPilgrims(ctx context.Context, operatorID, agentID string) ([]*domain.AgentPilgrim, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListMyPilgrims(ctx, db.ListMyPilgrimsParams{AgentID: agentUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.AgentPilgrim, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.AgentPilgrim{
			ID: uuidString(row.ID), FullName: row.FullName, PassportNumber: row.PassportNumber, Gender: row.Gender,
			PaymentStatus: row.PaymentStatus, DocsComplete: row.DocsComplete.Bool, PilgrimStatus: row.PilgrimStatus,
			SeasonID: uuidString(row.SeasonID), SeasonName: row.SeasonName, DepartureDate: timestamptzPtr(row.DepartureDate),
		})
	}
	return result, nil
}

// ListCommissionEntries returns the agent's commission ledger, newest first.
// Earnings and reversals both appear, so the list accounts for the balance.
func (r *AgentRepository) ListCommissionEntries(ctx context.Context, agentID string) ([]*domain.CommissionEntry, error) {
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListCommissionEntriesForAgent(ctx, agentUUID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.CommissionEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.CommissionEntry{
			ID:          uuid.UUID(row.ID.Bytes).String(),
			AmountIDR:   row.AmountIdr,
			Kind:        row.Kind,
			Note:        row.Note,
			ProductName: row.ProductName,
			CreatedAt:   row.CreatedAt.Time,
		})
	}
	return result, nil
}

// ListReferredCustomerRecap returns one row per jamaah this agent referred who
// has transacted, with amounts net of refunds.
func (r *AgentRepository) ListReferredCustomerRecap(ctx context.Context, operatorID, agentID string) ([]*domain.ReferredCustomerRecap, error) {
	operatorUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListReferredCustomerRecapForAgent(ctx, db.ListReferredCustomerRecapForAgentParams{
		OperatorID: operatorUUID, AgentID: agentUUID,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.ReferredCustomerRecap, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.ReferredCustomerRecap{
			PilgrimID: uuid.UUID(row.PilgrimID.Bytes).String(), PilgrimName: row.PilgrimName,
			OrderCount: row.OrderCount, RefundedOrderCount: row.RefundedOrderCount,
			TotalPaidIDR: row.TotalPaidIdr, RefundedIDR: row.RefundedIdr,
			CommissionIDR: row.CommissionIdr, LastTransactionAt: row.LastTransactionAt.Time,
		})
	}
	return result, nil
}

func (r *AgentRepository) SumPendingRequests(ctx context.Context, agentID string) (int64, error) {
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return 0, err
	}
	return r.queries.SumPendingPayoutRequests(ctx, agentUUID)
}

// The Tx variants let the caller hold a lock across the balance read and the
// insert. Requesting a payout is a check-then-act over money, so those steps
// have to see a consistent view — see AgentService.RequestPayout.
func (r *AgentRepository) GetPayoutSummaryTx(ctx context.Context, tx pgx.Tx, operatorID, agentID string) (*domain.AgentPayout, error) {
	return r.payoutSummary(ctx, r.queries.WithTx(tx), operatorID, agentID)
}

func (r *AgentRepository) SumPendingRequestsTx(ctx context.Context, tx pgx.Tx, agentID string) (int64, error) {
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return 0, err
	}
	return r.queries.WithTx(tx).SumPendingPayoutRequests(ctx, agentUUID)
}

func (r *AgentRepository) CreatePayoutRequestTx(ctx context.Context, tx pgx.Tx, operatorID, agentID string, amountIDR int64, note string) (*domain.PayoutRequest, error) {
	return r.createPayoutRequest(ctx, r.queries.WithTx(tx), operatorID, agentID, amountIDR, note)
}

func (r *AgentRepository) CreatePayoutRequest(ctx context.Context, operatorID, agentID string, amountIDR int64, note string) (*domain.PayoutRequest, error) {
	return r.createPayoutRequest(ctx, r.queries, operatorID, agentID, amountIDR, note)
}

func (r *AgentRepository) createPayoutRequest(ctx context.Context, q *db.Queries, operatorID, agentID string, amountIDR int64, note string) (*domain.PayoutRequest, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	row, err := q.CreatePayoutRequest(ctx, db.CreatePayoutRequestParams{OperatorID: opUUID, AgentID: agentUUID, AmountIdr: amountIDR, Note: note})
	if err != nil {
		return nil, err
	}
	return toPayoutRequest(row, ""), nil
}

func (r *AgentRepository) GetPayoutRequest(ctx context.Context, operatorID, requestID string) (*domain.PayoutRequest, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	reqUUID, err := pgUUID(requestID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetPayoutRequest(ctx, db.GetPayoutRequestParams{ID: reqUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	return &domain.PayoutRequest{
		ID: uuid.UUID(row.ID.Bytes).String(), AgentID: uuid.UUID(row.AgentID.Bytes).String(), AgentName: row.AgentName,
		AmountIDR: row.AmountIdr, Note: row.Note, Status: row.Status, ResolutionNote: row.ResolutionNote,
		RequestedAt: row.RequestedAt.Time, ResolvedAt: pgTimeToPtr(row.ResolvedAt),
	}, nil
}

// ListPayoutRequests returns PENDING requests only — approved/rejected ones
// are done, not something an operator or agent needs surfaced as a queue
// item. agentID empty lists every pending request across the operator (the
// requests inbox); set to scope to one agent (e.g. inside the payout
// dialog, or an agent's own wallet).
func (r *AgentRepository) ListPayoutRequests(ctx context.Context, operatorID, agentID string) ([]*domain.PayoutRequest, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	var agentFilter pgtype.UUID
	if agentID != "" {
		agentFilter, err = pgUUID(agentID)
		if err != nil {
			return nil, err
		}
	}
	rows, err := r.queries.ListPayoutRequests(ctx, db.ListPayoutRequestsParams{OperatorID: opUUID, AgentID: agentFilter})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.PayoutRequest, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.PayoutRequest{
			ID: uuid.UUID(row.ID.Bytes).String(), AgentID: uuid.UUID(row.AgentID.Bytes).String(), AgentName: row.AgentName,
			AmountIDR: row.AmountIdr, Note: row.Note, Status: row.Status, ResolutionNote: row.ResolutionNote,
			RequestedAt: row.RequestedAt.Time, ResolvedAt: pgTimeToPtr(row.ResolvedAt),
		})
	}
	return result, nil
}

// ApprovePayoutRequestTx is only ever called alongside RecordPayoutTx, in
// the same transaction — a request must never end up APPROVED without a
// matching ledger entry, or vice versa.
func (r *AgentRepository) ApprovePayoutRequestTx(ctx context.Context, tx pgx.Tx, operatorID, requestID, resolvedByUserID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	reqUUID, err := pgUUID(requestID)
	if err != nil {
		return err
	}
	_, err = r.queries.WithTx(tx).ApprovePayoutRequestTx(ctx, db.ApprovePayoutRequestTxParams{ID: reqUUID, OperatorID: opUUID, ResolvedByUserID: pgtype.Text{String: resolvedByUserID, Valid: true}})
	return err
}

func (r *AgentRepository) RejectPayoutRequest(ctx context.Context, operatorID, requestID, resolvedByUserID, note string) (*domain.PayoutRequest, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	reqUUID, err := pgUUID(requestID)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.RejectPayoutRequest(ctx, db.RejectPayoutRequestParams{ID: reqUUID, OperatorID: opUUID, ResolvedByUserID: pgtype.Text{String: resolvedByUserID, Valid: true}, ResolutionNote: note})
	if err != nil {
		return nil, err
	}
	return toPayoutRequest(row, ""), nil
}

func toPayoutRequest(row db.AgentPayoutRequest, agentName string) *domain.PayoutRequest {
	return &domain.PayoutRequest{
		ID: uuid.UUID(row.ID.Bytes).String(), AgentID: uuid.UUID(row.AgentID.Bytes).String(), AgentName: agentName,
		AmountIDR: row.AmountIdr, Note: row.Note, Status: row.Status, ResolutionNote: row.ResolutionNote,
		RequestedAt: row.RequestedAt.Time, ResolvedAt: pgTimeToPtr(row.ResolvedAt),
	}
}

func pgTimeToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func (r *AgentRepository) ListPayoutHistory(ctx context.Context, operatorID, agentID string) ([]*domain.AgentPayoutEntry, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListAgentPayoutHistory(ctx, db.ListAgentPayoutHistoryParams{AgentID: agentUUID, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.AgentPayoutEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.AgentPayoutEntry{
			ID:         uuid.UUID(row.ID.Bytes).String(),
			AmountIDR:  row.AmountIdr,
			Note:       row.Note,
			Method:     row.Method,
			PaidByName: row.PaidByName,
			CreatedAt:  row.CreatedAt.Time,
		})
	}
	return result, nil
}

func (r *AgentRepository) Update(ctx context.Context, operatorID, agentID, name, phone, email, notes string, commissionRate float64, isActive bool) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return nil, err
	}
	agent, err := r.queries.UpdateAgent(ctx, db.UpdateAgentParams{ID: agentUUID, OperatorID: opUUID, Name: name, Phone: phone, Email: email, CommissionRate: commissionRate, Notes: notes, IsActive: isActive})
	if err != nil {
		return nil, err
	}
	return toAgent(agent, 0), nil
}

func (r *AgentRepository) CreateApplication(ctx context.Context, operatorID, name, phone, email, referredByAgentID string) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agent, err := r.queries.CreateAgentApplication(ctx, db.CreateAgentApplicationParams{OperatorID: opUUID, Name: name, Phone: phone, Email: email, Column5: referredByAgentID})
	if err != nil {
		return nil, err
	}
	return toAgent(agent, 0), nil
}

func (r *AgentRepository) GetByReferralCode(ctx context.Context, operatorID, referralCode string) (*domain.Agent, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	agent, err := r.queries.GetAgentByReferralCode(ctx, db.GetAgentByReferralCodeParams{ReferralCode: referralCode, OperatorID: opUUID})
	if err != nil {
		return nil, err
	}
	return toAgent(agent, 0), nil
}

func (r *AgentRepository) ListActiveForTiering(ctx context.Context, operatorID string) ([]db.ListActiveAgentsForTieringRow, error) {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return nil, err
	}
	return r.queries.ListActiveAgentsForTiering(ctx, opUUID)
}

func (r *AgentRepository) UpdateTier(ctx context.Context, operatorID, agentID, tier string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return err
	}
	return r.queries.UpdateAgentTier(ctx, db.UpdateAgentTierParams{ID: agentUUID, OperatorID: opUUID, Tier: tier})
}

func (r *AgentRepository) Delete(ctx context.Context, operatorID, agentID string) error {
	opUUID, err := pgUUID(operatorID)
	if err != nil {
		return err
	}
	agentUUID, err := pgUUID(agentID)
	if err != nil {
		return err
	}
	return r.queries.DeleteAgent(ctx, db.DeleteAgentParams{ID: agentUUID, OperatorID: opUUID})
}

func toAgent(agent db.Agent, pilgrimCount int32) *domain.Agent {
	referredBy := ""
	if agent.ReferredByAgentID.Valid {
		referredBy = uuid.UUID(agent.ReferredByAgentID.Bytes).String()
	}
	return &domain.Agent{
		ID: uuid.UUID(agent.ID.Bytes).String(), OperatorID: uuid.UUID(agent.OperatorID.Bytes).String(), Name: agent.Name, Phone: agent.Phone, Email: agent.Email,
		CommissionRate: agent.CommissionRate, Notes: agent.Notes, IsActive: agent.IsActive, PilgrimCount: pilgrimCount, ReferralCode: agent.ReferralCode, Tier: agent.Tier,
		ReferredByAgentID: referredBy, CreatedAt: agent.CreatedAt.Time, UpdatedAt: agent.UpdatedAt.Time,
		NIK: agent.Nik, NPWP: agent.Npwp, Address: agent.Address, DateOfBirth: datePtr(agent.DateOfBirth), PassportNumber: agent.PassportNumber,
		PassportExpiryDate: datePtr(agent.PassportExpiryDate), BankName: agent.BankName, BankAccountNumber: agent.BankAccountNumber, BankAccountHolder: agent.BankAccountHolder,
		KYCStatus: agent.KycStatus, KYCSource: agent.KycSource, KYCVerifiedBy: agent.KycVerifiedBy, KYCVerifiedAt: timestamptzPtr(agent.KycVerifiedAt), KYCRejectionReason: agent.KycRejectionReason,
	}
}
