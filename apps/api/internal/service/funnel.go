package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/funnel"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
)

// FunnelService records visitor steps.
//
// Every failure here is swallowed. A visit that goes uncounted costs one row in
// a report; a storefront that fails to render because analytics was unhappy
// costs the travel agency a customer.
type FunnelService struct {
	repository *repository.FunnelRepository
	hasher     *funnel.Hasher
}

func NewFunnelService(funnelRepository *repository.FunnelRepository, hasher *funnel.Hasher) *FunnelService {
	return &FunnelService{repository: funnelRepository, hasher: hasher}
}

// RecordEvent takes the client address and user agent explicitly rather than
// digging them out of the context: they come from request headers, which belong
// to the handler, and passing them makes the bot and token behaviour testable
// without building a request.
func (s *FunnelService) RecordEvent(ctx context.Context, req *hajjv1.RecordEventRequest, clientIP, userAgent string) (*hajjv1.RecordEventResponse, error) {
	empty := &hajjv1.RecordEventResponse{}
	if req == nil {
		return empty, nil
	}
	// No salt, no recording. Writing a token that cannot be trusted to be
	// anonymous would turn this table into personal data without anybody
	// deciding to.
	if !s.hasher.Configured() {
		return empty, nil
	}

	if funnel.IsBot(userAgent) {
		return empty, nil
	}

	operatorID := ""
	if id := strings.TrimSpace(req.OperatorId); id != "" {
		// Checked, not trusted. An id nobody owns would otherwise land as the
		// platform's own visit and inflate the one funnel with no cross-check.
		if !s.repository.OperatorExists(ctx, id) {
			return empty, nil
		}
		operatorID = id
	}
	if slug := strings.TrimSpace(req.OperatorSlug); operatorID == "" && slug != "" {
		resolved, err := s.repository.OperatorIDBySlug(ctx, slug)
		if err != nil {
			return empty, nil
		}
		// An unknown slug means the row has no owner, and a row with no owner
		// would be counted as the platform's own traffic. Dropped instead.
		if resolved == "" {
			return empty, nil
		}
		operatorID = resolved
	}

	visitor := s.hasher.Visitor(clientIP, userAgent)
	if visitor == "" {
		return empty, nil
	}

	_ = s.repository.Record(ctx, repository.FunnelEvent{
		OperatorID:   operatorID,
		VisitorHash:  visitor,
		Step:         req.Step,
		Path:         req.Path,
		ArticleSlug:  req.ArticleSlug,
		ReferrerHost: req.ReferrerHost,
		UTMSource:    req.UtmSource,
		UTMCampaign:  req.UtmCampaign,
	})
	return empty, nil
}

// Record implements funnel.Recorder for callers that are doing something else
// and want the step counted alongside it.
//
// Every failure is dropped on purpose: the caller is completing a registration
// or a signup, and neither may be jeopardised by a row in a report.
func (s *FunnelService) Record(ctx context.Context, step funnel.Step) {
	if !s.hasher.Configured() || funnel.IsBot(step.UserAgent) {
		return
	}
	visitor := s.hasher.Visitor(step.ClientIP, step.UserAgent)
	if visitor == "" {
		return
	}
	_ = s.repository.Record(ctx, repository.FunnelEvent{
		OperatorID:  step.OperatorID,
		VisitorHash: visitor,
		Step:        step.Step,
		Path:        step.Path,
		UTMSource:   step.UTMSource,
		UTMCampaign: step.UTMCampaign,
		EntityID:    step.EntityID,
	})
}
