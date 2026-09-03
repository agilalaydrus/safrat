package middleware

import (
	"context"
	"log/slog"
	"strings"

	"github.com/hajj-saas/api/internal/repository"
)

// personalDataServices are the surfaces that return information about
// identifiable people: names, dates of birth, passport and identity numbers,
// phone numbers, addresses, health notes, locations, and messages.
//
// Listed by service rather than by procedure, and read as a prefix, so a new
// RPC on one of these services is covered the day it is added rather than the
// day somebody remembers to list it. The cost of over-recording is a row that
// says somebody looked at a screen; the cost of under-recording is a read of
// somebody's passport number that nothing can attribute afterwards.
var personalDataServices = []string{
	"/hajj.v1.PilgrimService/",
	"/hajj.v1.PilgrimAppService/",
	"/hajj.v1.RegistrationService/",
	"/hajj.v1.KycService/",
	"/hajj.v1.HealthReportService/",
	"/hajj.v1.DocumentService/",
	"/hajj.v1.ChatService/",
	"/hajj.v1.SOSService/",
	"/hajj.v1.GroupLeaderService/",
	"/hajj.v1.MonitoringService/",
	"/hajj.v1.LostReportService/",
	"/hajj.v1.WaitlistService/",
	"/hajj.v1.InsuranceService/",
}

// IsPersonalDataRead reports whether this procedure hands back information
// about identifiable people. Only reads count: a write already leaves its own
// audit entry, and recording it twice would say two things happened.
func IsPersonalDataRead(procedure string) bool {
	if !ImpersonationAllows(procedure) {
		// Reuses the read classifier deliberately. Anything it refuses is
		// either a write, which is audited elsewhere, or a surface nobody
		// should reach this way.
		return false
	}
	for _, service := range personalDataServices {
		if strings.HasPrefix(procedure, service) {
			return true
		}
	}
	return false
}

// recordPersonalDataRead notes the read without standing in its way.
//
// Best effort on purpose: a support person must not be shown an error because
// the bookkeeping failed, and a customer's data must not become unreadable
// because a log table is full. The failure is logged loudly instead, because a
// silent gap in this table is worse than a noisy one.
func (a *authInterceptor) recordPersonalDataRead(ctx context.Context, procedure, actorUserID, impersonationID, operatorID string) {
	if a.personalDataReads == nil || !IsPersonalDataRead(procedure) {
		return
	}
	if err := a.personalDataReads.Record(ctx, repository.PersonalDataRead{
		ActorUserID: actorUserID, ImpersonationID: impersonationID,
		OperatorID: operatorID, Procedure: procedure,
	}); err != nil {
		slog.Error("record personal data read", "procedure", procedure, "error", err)
	}
}
