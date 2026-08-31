package service

import (
	"context"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
	"strings"
)

type BranchService struct {
	operators   *repository.OperatorRepository
	branches    *repository.BranchRepository
	entitlement *EntitlementChecker
}

func NewBranchService(o *repository.OperatorRepository, b *repository.BranchRepository, e *EntitlementChecker) *BranchService {
	return &BranchService{operators: o, branches: b, entitlement: e}
}
func (s *BranchService) operator(ctx context.Context, org string) (string, error) {
	o, e := s.operators.GetByBetterAuthOrgID(ctx, org)
	if e != nil {
		return "", e
	}
	return o.ID, nil
}
func branchInput(r *hajjv1.CreateBranchRequest) domain.Branch {
	return domain.Branch{Name: strings.TrimSpace(r.Name), City: strings.TrimSpace(r.City), TargetPilgrims: r.TargetPilgrims, TargetRevenueIDR: r.TargetRevenueIdr, Phone: strings.TrimSpace(r.Phone), BankName: strings.TrimSpace(r.BankName), AccountNumber: strings.TrimSpace(r.AccountNumber), AccountHolder: strings.TrimSpace(r.AccountHolder), IsActive: true}
}
func (s *BranchService) List(ctx context.Context, org string, inactive bool) (*hajjv1.ListBranchesResponse, error) {
	id, e := s.operator(ctx, org)
	if e != nil {
		return nil, e
	}
	rows, e := s.branches.List(ctx, id, inactive)
	if e != nil {
		return nil, e
	}
	out := &hajjv1.ListBranchesResponse{Branches: make([]*hajjv1.Branch, 0, len(rows))}
	for _, v := range rows {
		out.Branches = append(out.Branches, branchMessage(v))
	}
	return out, nil
}
func (s *BranchService) Create(ctx context.Context, org string, r *hajjv1.CreateBranchRequest) (*hajjv1.Branch, error) {
	if r == nil || strings.TrimSpace(r.Name) == "" {
		return nil, apperror.ErrValidation
	}
	id, e := s.operator(ctx, org)
	if e != nil {
		return nil, e
	}
	if e = s.entitlement.Check(ctx, id, "branches"); e != nil {
		return nil, e
	}
	v, e := s.branches.Create(ctx, id, branchInput(r))
	if e != nil {
		return nil, e
	}
	return branchMessage(*v), nil
}
func (s *BranchService) Update(ctx context.Context, org string, r *hajjv1.UpdateBranchRequest) (*hajjv1.Branch, error) {
	if r == nil || !isUUID(r.BranchId) || strings.TrimSpace(r.Name) == "" {
		return nil, apperror.ErrValidation
	}
	id, e := s.operator(ctx, org)
	if e != nil {
		return nil, e
	}
	v, e := s.branches.Update(ctx, id, domain.Branch{ID: r.BranchId, Name: strings.TrimSpace(r.Name), City: strings.TrimSpace(r.City), TargetPilgrims: r.TargetPilgrims, TargetRevenueIDR: r.TargetRevenueIdr, Phone: r.Phone, BankName: r.BankName, AccountNumber: r.AccountNumber, AccountHolder: r.AccountHolder, IsActive: r.IsActive})
	if e != nil {
		return nil, e
	}
	return branchMessage(*v), nil
}
func (s *BranchService) AssignHead(ctx context.Context, org string, r *hajjv1.AssignBranchHeadRequest) (*hajjv1.Branch, error) {
	if r == nil || !isUUID(r.BranchId) {
		return nil, apperror.ErrValidation
	}
	id, e := s.operator(ctx, org)
	if e != nil {
		return nil, e
	}
	if e = s.branches.AssignHead(ctx, id, r.BranchId, strings.TrimSpace(r.UserId)); e != nil {
		return nil, e
	}
	rows, e := s.branches.List(ctx, id, true)
	if e != nil {
		return nil, e
	}
	for _, v := range rows {
		if v.ID == r.BranchId {
			return branchMessage(v), nil
		}
	}
	return nil, apperror.ErrNotFound
}
func (s *BranchService) Performance(ctx context.Context, org string, r *hajjv1.GetBranchPerformanceRequest) (*hajjv1.GetBranchPerformanceResponse, error) {
	if r == nil || !isUUID(r.SeasonId) {
		return nil, apperror.ErrValidation
	}
	id, e := s.operator(ctx, org)
	if e != nil {
		return nil, e
	}
	report, e := s.branches.GetPerformance(ctx, id, r.SeasonId)
	if e != nil {
		return nil, e
	}
	out := &hajjv1.GetBranchPerformanceResponse{NetworkRevenueIdr: report.NetworkRevenueIDR, NetworkPilgrimCount: report.NetworkPilgrimCount, BelowTargetCount: report.BelowTargetCount, Branches: make([]*hajjv1.BranchPerformance, 0, len(report.Branches))}
	for _, v := range report.Branches {
		p := &hajjv1.BranchPerformance{Branch: &hajjv1.Branch{Id: v.BranchID, Name: v.Name, City: v.City, TargetPilgrims: v.TargetPilgrims, TargetRevenueIdr: v.TargetRevenueIDR}, RevenueIdr: v.RevenueIDR, PilgrimCount: v.PilgrimCount, AgentCount: v.AgentCount, RevenueAchievementPct: v.RevenueAchievementPct, PilgrimAchievementPct: v.PilgrimAchievementPct, Score: v.Score, CollectionPct: v.CollectionPct, DocumentsReadyPct: v.DocumentsReadyPct}
		for _, t := range v.Trend {
			p.Trend = append(p.Trend, &hajjv1.BranchTrendPoint{Month: t.Month, RevenueIdr: t.RevenueIDR, PilgrimCount: t.PilgrimCount})
		}
		out.Branches = append(out.Branches, p)
	}
	return out, nil
}
func branchMessage(v domain.Branch) *hajjv1.Branch {
	return &hajjv1.Branch{Id: v.ID, OperatorId: v.OperatorID, Name: v.Name, City: v.City, TargetPilgrims: v.TargetPilgrims, TargetRevenueIdr: v.TargetRevenueIDR, HeadUserId: v.HeadUserID, Phone: v.Phone, BankName: v.BankName, AccountNumber: v.AccountNumber, AccountHolder: v.AccountHolder, IsActive: v.IsActive, CreatedAt: timestamppb.New(v.CreatedAt), UpdatedAt: timestamppb.New(v.UpdatedAt)}
}
