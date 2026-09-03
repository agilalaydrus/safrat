package service

import (
	"context"

	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
)

// FunnelReportService reads one operator's funnel.
//
// The operator is resolved from the caller's own session and never taken from
// the request. A travel agency reading another's funnel would be unauthorised
// access to their commercial data, so there is deliberately no parameter that
// could carry somebody else's id.
type FunnelReportService struct {
	operatorRepository *repository.OperatorRepository
	funnelRepository   *repository.FunnelRepository
	retentionDays      int
}

func NewFunnelReportService(operators *repository.OperatorRepository, funnelRepository *repository.FunnelRepository, retentionDays int) *FunnelReportService {
	return &FunnelReportService{operatorRepository: operators, funnelRepository: funnelRepository, retentionDays: retentionDays}
}

func (s *FunnelReportService) GetReport(ctx context.Context, authenticatedOrgID string, req *hajjv1.GetFunnelReportRequest) (*hajjv1.FunnelReport, error) {
	operator, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, authenticatedOrgID)
	if err != nil {
		return nil, serviceError("FunnelReportService.GetReport", err)
	}
	days := int32(30)
	if req != nil && req.Days > 0 {
		days = req.Days
	}
	report, err := s.funnelRepository.Report(ctx, operator.ID, days)
	if err != nil {
		return nil, serviceError("FunnelReportService.GetReport", err)
	}

	message := &hajjv1.FunnelReport{Days: days, RawRetentionDays: int32(s.retentionDays)}
	for _, step := range report.Steps {
		message.Steps = append(message.Steps, &hajjv1.FunnelStepCount{Step: step.Step, Visitors: step.Visitors, Events: step.Events})
	}
	for _, source := range report.Sources {
		message.Sources = append(message.Sources, &hajjv1.FunnelSourceCount{Source: source.Source, Visitors: source.Visitors, Registrations: source.Registrations})
	}
	for _, hour := range report.Hours {
		message.Hours = append(message.Hours, &hajjv1.FunnelHourCount{Hour: hour.Hour, Visitors: hour.Visitors})
	}
	for _, place := range report.Places {
		message.Places = append(message.Places, &hajjv1.FunnelPlaceCount{Province: place.Province, City: place.City, Visitors: place.Visitors})
	}
	for _, article := range report.Articles {
		message.Articles = append(message.Articles, &hajjv1.FunnelArticleCount{Slug: article.Slug, Readers: article.Readers, Registrations: article.Registrations})
	}
	for _, day := range report.Daily {
		message.Daily = append(message.Daily, &hajjv1.FunnelDayCount{Day: day.Day, Visitors: day.Visitors, Registrations: day.Registrations})
	}
	for _, age := range report.ChannelAges {
		message.ChannelAges = append(message.ChannelAges, &hajjv1.FunnelChannelAge{Source: age.Source, AverageAge: age.AverageAge, Sample: age.Sample})
	}
	return message, nil
}
