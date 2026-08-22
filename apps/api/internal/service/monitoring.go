package service

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/events"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MonitoringService struct {
	operatorRepository   *repository.OperatorRepository
	monitoringRepository *repository.MonitoringRepository
	groupRepository      *repository.GroupRepository
	bus                  *events.Bus
}

func NewMonitoringService(operators *repository.OperatorRepository, monitoring *repository.MonitoringRepository, groups *repository.GroupRepository, bus *events.Bus) *MonitoringService {
	return &MonitoringService{operatorRepository: operators, monitoringRepository: monitoring, groupRepository: groups, bus: bus}
}

func (s *MonitoringService) GetSnapshot(ctx context.Context, orgID string, req *hajjv1.GetSnapshotRequest) (*hajjv1.MonitoringSnapshot, error) {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return nil, serviceError("MonitoringService.GetSnapshot", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("MonitoringService.GetSnapshot", err)
	}

	sos, err := s.monitoringRepository.ListActiveSOS(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("MonitoringService.GetSnapshot", err)
	}
	health, err := s.monitoringRepository.ListOpenHealthReports(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("MonitoringService.GetSnapshot", err)
	}
	groups, err := s.groupRepository.ListForOperator(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("MonitoringService.GetSnapshot", err)
	}
	ritualProgress, err := s.monitoringRepository.ListGroupRitualProgress(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("MonitoringService.GetSnapshot", err)
	}
	timeline, err := s.monitoringRepository.ListReturnTimeline(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("MonitoringService.GetSnapshot", err)
	}

	sosGroupIDs := make(map[string]bool, len(sos))
	for _, a := range sos {
		if a.GroupID != "" {
			sosGroupIDs[a.GroupID] = true
		}
	}

	result := &hajjv1.MonitoringSnapshot{GeneratedAt: timestamppb.Now()}
	for _, a := range sos {
		result.ActiveSos = append(result.ActiveSos, &hajjv1.SOSAlertMini{Id: a.ID, PilgrimName: a.PilgrimName, GroupName: a.GroupName, Status: a.Status, CreatedAt: timestamppb.New(a.CreatedAt)})
	}
	for _, h := range health {
		result.OpenHealthReports = append(result.OpenHealthReports, &hajjv1.HealthReportMini{Id: h.ID, PilgrimName: h.PilgrimName, GroupName: h.GroupName, Severity: h.Severity, Symptoms: h.Symptoms, CreatedAt: timestamppb.New(h.CreatedAt)})
	}
	for _, g := range groups {
		card := &hajjv1.GroupMonitoringCard{
			GroupId: g.ID, Name: g.Name, LeaderName: g.LeaderName, CurrentCity: g.CurrentCity, CurrentActivity: g.CurrentActivity,
			PilgrimCount: g.PilgrimCount, RitualCompletionPct: -1, HasActiveSos: sosGroupIDs[g.ID],
		}
		if g.LastUpdate != nil {
			card.LastUpdate = timestamppb.New(*g.LastUpdate)
		}
		if p, ok := ritualProgress[g.ID]; ok && p.TemplateCount > 0 && p.PilgrimCount > 0 {
			card.RitualCompletionPct = float64(p.CompletedCount) / float64(p.TemplateCount*p.PilgrimCount)
		}
		result.Groups = append(result.Groups, card)
	}
	for _, t := range timeline {
		result.ReturnTimeline = append(result.ReturnTimeline, &hajjv1.ReturnTimelineItem{KloterId: t.KloterID, KloterCode: t.KloterCode, ReturnAt: timestamppb.New(t.ReturnAt), TotalPilgrims: t.TotalPilgrims, ReadyCount: t.ReadyCount})
	}
	return result, nil
}

// StreamEvents holds the connection open until the client disconnects
// (ctx.Done()) or the bus's per-operator subscriber cap is hit. Never
// carries pilgrim data — see monitoring.proto's doc comment on
// MonitoringPing; the client always refetches GetSnapshot on receipt.
func (s *MonitoringService) StreamEvents(ctx context.Context, orgID string, req *hajjv1.StreamEventsRequest, stream *connect.ServerStream[hajjv1.MonitoringPing]) error {
	if req == nil || strings.TrimSpace(req.SeasonId) == "" {
		return serviceError("MonitoringService.StreamEvents", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("MonitoringService.StreamEvents", err)
	}
	ch, unsubscribe, ok := s.bus.Subscribe(op.ID)
	if !ok {
		return serviceError("MonitoringService.StreamEvents", apperror.ErrFailedPrecondition)
	}
	defer unsubscribe()

	// An initial ping immediately after subscribing lets the client do one
	// GetSnapshot call and then rely entirely on the stream, instead of
	// racing a separate initial fetch against stream setup.
	if err := stream.Send(&hajjv1.MonitoringPing{Type: "connected", At: timestamppb.Now()}); err != nil {
		return err
	}

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, open := <-ch:
			if !open {
				return nil
			}
			if err := stream.Send(&hajjv1.MonitoringPing{Type: ev.Type, EntityId: ev.EntityID, At: timestamppb.New(ev.CreatedAt)}); err != nil {
				return err
			}
		case <-heartbeat.C:
			// Keeps the connection alive through proxies/load balancers that
			// close idle long-lived HTTP responses, and lets the client
			// detect a silently-dead connection via a read timeout.
			if err := stream.Send(&hajjv1.MonitoringPing{Type: "heartbeat", At: timestamppb.Now()}); err != nil {
				return err
			}
		}
	}
}
