package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/domain"
	db "github.com/hajj-saas/api/internal/gen/db"
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
	return &domain.Agent{ID: uuid.UUID(agent.ID.Bytes).String(), OperatorID: uuid.UUID(agent.OperatorID.Bytes).String(), Name: agent.Name, Phone: agent.Phone, Email: agent.Email, CommissionRate: agent.CommissionRate, Notes: agent.Notes, IsActive: agent.IsActive, PilgrimCount: pilgrimCount, ReferralCode: agent.ReferralCode, Tier: agent.Tier, ReferredByAgentID: referredBy, CreatedAt: agent.CreatedAt.Time, UpdatedAt: agent.UpdatedAt.Time}
}
