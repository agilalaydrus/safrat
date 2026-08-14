package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AgentService struct {
	operatorRepository *repository.OperatorRepository
	agentRepository    *repository.AgentRepository
}

func NewAgentService(operators *repository.OperatorRepository, agents *repository.AgentRepository) *AgentService {
	return &AgentService{operatorRepository: operators, agentRepository: agents}
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
	return &hajjv1.Agent{Id: agent.ID, OperatorId: agent.OperatorID, Name: agent.Name, Phone: agent.Phone, Email: agent.Email, CommissionRate: agent.CommissionRate, Notes: agent.Notes, IsActive: agent.IsActive, PilgrimCount: agent.PilgrimCount, CreatedAt: timestamppb.New(agent.CreatedAt), UpdatedAt: timestamppb.New(agent.UpdatedAt)}
}
