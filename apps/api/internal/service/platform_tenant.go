package service

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GetTenantDetail is one travel agency's whole picture in one call: what they
// pay, what they use, who is on the team, and what has been done to them.
func (s *PlatformService) GetTenantDetail(ctx context.Context, req *hajjv1.GetTenantDetailRequest) (*hajjv1.GetTenantDetailResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) {
		// An id that is not an id is a not-found, not an internal fault: this
		// page is reached by URL, and a mistyped one should say so.
		return nil, connect.NewError(connect.CodeNotFound, errors.New("travel tidak ditemukan"))
	}
	detail, err := s.platformRepository.GetTenantDetail(ctx, req.OperatorId)
	if errors.Is(err, repository.ErrTenantNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("travel tidak ditemukan"))
	}
	if err != nil {
		return nil, serviceError("PlatformService.GetTenantDetail", err)
	}

	operator := &hajjv1.PlatformOperator{
		Id: detail.Operator.ID, Name: detail.Operator.Name, Slug: detail.Operator.Slug,
		Plan: detail.Operator.Plan, SubscriptionStatus: detail.Operator.SubscriptionStatus,
		PilgrimCount: detail.Operator.PilgrimCount, ProductCount: detail.Operator.ProductCount,
		HeldOrderCount: detail.Operator.HeldOrderCount, CreatedAt: timestamppb.New(detail.Operator.CreatedAt),
		DunningStage: detail.Operator.DunningStage, OutstandingIdr: detail.Operator.OutstandingIDR,
		GracePeriodDays: detail.Operator.GracePeriodDays, GracePeriodOverrideDays: detail.Operator.GraceOverrideDays,
		CreditBalanceIdr: detail.Operator.CreditBalanceIDR,
	}
	if detail.Operator.AccessUntil != nil {
		operator.AccessUntil = timestamppb.New(*detail.Operator.AccessUntil)
	}
	if detail.Operator.SuspendedAt != nil {
		operator.SuspendedAt = timestamppb.New(*detail.Operator.SuspendedAt)
	}
	if detail.Operator.EffectiveAccessUntil != nil {
		operator.EffectiveAccessUntil = timestamppb.New(*detail.Operator.EffectiveAccessUntil)
	}

	response := &hajjv1.GetTenantDetailResponse{
		Operator: operator,
		Counts: &hajjv1.TenantCounts{
			Pilgrims: detail.Counts.Pilgrims, Branches: detail.Counts.Branches,
			Seasons: detail.Counts.Seasons, Products: detail.Counts.Products,
			Registrations: detail.Counts.Registrations, HeldOrders: detail.Counts.HeldOrders,
			KycPending: detail.Counts.KycPending, KycVerified: detail.Counts.KycVerified,
			Staff: detail.Counts.Staff,
		},
		Funnel: &hajjv1.StorefrontFunnelRow{
			OperatorId: detail.Funnel.OperatorID, OperatorName: detail.Funnel.OperatorName,
			Slug: detail.Funnel.Slug, Visitors: detail.Funnel.Visitors,
			Registrations: detail.Funnel.Registrations, Conversion: detail.Funnel.Conversion,
		},
	}

	for _, row := range detail.Usage {
		message := &hajjv1.UsageRow{
			OperatorId: row.OperatorID, OperatorName: row.OperatorName, Plan: row.Plan,
			Metric: row.Metric, Value: row.Value, ComputedAt: timestamppb.New(row.ComputedAt),
		}
		// Absent stays absent. Copying a zero here would tell the screen this
		// tenant may create nothing at all.
		if row.Limit != nil {
			message.Limit = row.Limit
		}
		response.Usage = append(response.Usage, message)
	}
	if detail.Override != nil {
		response.Override = platformPlanOverrideMessage(*detail.Override)
		response.HasOverride = true
	}
	for _, invoice := range detail.Invoices {
		row := &hajjv1.SubscriptionInvoiceRow{
			Id: invoice.ID, OperatorId: invoice.OperatorID, OperatorName: invoice.OperatorName,
			Plan: invoice.Plan, Status: invoice.Status, Channel: invoice.Channel,
			AmountIdr: invoice.AmountIDR, DueAt: timestamppb.New(invoice.DueAt),
			VoidedReason: invoice.VoidedReason, CreatedAt: timestamppb.New(invoice.CreatedAt),
		}
		if invoice.PaidAt != nil {
			row.PaidAt = timestamppb.New(*invoice.PaidAt)
		}
		if invoice.VoidedAt != nil {
			row.VoidedAt = timestamppb.New(*invoice.VoidedAt)
		}
		response.Invoices = append(response.Invoices, row)
	}
	for _, member := range detail.Team {
		row := &hajjv1.TenantTeamMember{
			UserId: member.UserID, Name: member.Name, Email: member.Email,
			Role: member.Role, TwoFactorEnabled: member.TwoFactorEnabled,
		}
		if member.JoinedAt != nil {
			row.JoinedAt = timestamppb.New(*member.JoinedAt)
		}
		response.Team = append(response.Team, row)
	}
	for _, domain := range detail.Domains {
		row := &hajjv1.TenantDomain{Hostname: domain.Hostname, IsPrimary: domain.IsPrimary}
		if domain.VerifiedAt != nil {
			row.VerifiedAt = timestamppb.New(*domain.VerifiedAt)
		}
		response.Domains = append(response.Domains, row)
	}
	for _, entry := range detail.Audit {
		response.Audit = append(response.Audit, &hajjv1.TenantAuditEntry{
			At: timestamppb.New(entry.At), Actor: entry.Actor, Action: entry.Action,
			EntityType: entry.EntityType, EntityId: entry.EntityID,
		})
	}
	return response, nil
}
