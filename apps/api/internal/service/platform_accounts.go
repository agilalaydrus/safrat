package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Accounts, platform access, and identity records — the things that previously
// required a SQL client, which is what this panel exists to remove.

func (s *PlatformService) ListAccounts(ctx context.Context, req *hajjv1.ListAccountsRequest) (*hajjv1.ListAccountsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	search := ""
	limit := int32(50)
	if req != nil {
		search = strings.TrimSpace(req.Search)
		if req.Limit > 0 {
			limit = req.Limit
		}
	}
	accounts, err := s.platformRepository.ListAccounts(ctx, search, limit)
	if err != nil {
		return nil, serviceError("PlatformService.ListAccounts", err)
	}
	result := &hajjv1.ListAccountsResponse{Accounts: make([]*hajjv1.PlatformAccount, 0, len(accounts))}
	for _, account := range accounts {
		result.Accounts = append(result.Accounts, &hajjv1.PlatformAccount{
			UserId: account.UserID, Name: account.Name, Email: account.Email,
			EmailVerified: account.EmailVerified, TwoFactorEnabled: account.TwoFactorEnabled,
			IsPlatformAdmin: account.IsPlatformAdmin, OperatorName: account.OperatorName,
			OrgRole: account.OrgRole, ActiveSessions: account.ActiveSessions,
			CreatedAt: timestamppb.New(account.CreatedAt),
		})
	}
	return result, nil
}

func (s *PlatformService) GrantPlatformAdmin(ctx context.Context, req *hajjv1.GrantPlatformAdminRequest) (*hajjv1.GrantPlatformAdminResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.UserId) == "" {
		return nil, serviceError("PlatformService.GrantPlatformAdmin", apperror.ErrValidation)
	}
	if err := s.platformRepository.GrantPlatformAdmin(ctx, req.UserId, req.Note, userID); err != nil {
		return nil, serviceError("PlatformService.GrantPlatformAdmin", err)
	}
	// The widest privilege in the system. Who granted it to whom, and why, is
	// the only record of how somebody came to hold it.
	s.auditPlatform(ctx, userID, "platform_admin_granted", req.UserId,
		"Akses admin platform diberikan"+noteSuffix(req.Note))
	return &hajjv1.GrantPlatformAdminResponse{}, nil
}

func (s *PlatformService) RevokePlatformAdmin(ctx context.Context, req *hajjv1.RevokePlatformAdminRequest) (*hajjv1.RevokePlatformAdminResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.UserId) == "" {
		return nil, serviceError("PlatformService.RevokePlatformAdmin", apperror.ErrValidation)
	}
	// Removing the last admin would lock the panel for everybody, and the only
	// way back would be the SQL client this panel exists to replace. Checked
	// before the delete rather than after, so it is refused rather than undone.
	remaining, err := s.platformRepository.CountPlatformAdmins(ctx)
	if err != nil {
		return nil, serviceError("PlatformService.RevokePlatformAdmin", err)
	}
	if remaining <= 1 {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("tidak dapat mencabut admin platform terakhir; tunjuk penggantinya lebih dulu"))
	}
	if _, err := s.platformRepository.RevokePlatformAdmin(ctx, req.UserId); err != nil {
		return nil, serviceError("PlatformService.RevokePlatformAdmin", err)
	}
	s.auditPlatform(ctx, userID, "platform_admin_revoked", req.UserId, "Akses admin platform dicabut")
	return &hajjv1.RevokePlatformAdminResponse{}, nil
}

// RevokeSessions signs an account out everywhere.
//
// The response to a suspected takeover: resetting a password changes nothing
// for whoever already holds a live session, and nothing else in this system
// ends one early.
func (s *PlatformService) RevokeSessions(ctx context.Context, req *hajjv1.RevokeSessionsRequest) (*hajjv1.RevokeSessionsResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.UserId) == "" {
		return nil, serviceError("PlatformService.RevokeSessions", apperror.ErrValidation)
	}
	ended, err := s.platformRepository.RevokeSessions(ctx, req.UserId)
	if err != nil {
		return nil, serviceError("PlatformService.RevokeSessions", err)
	}
	s.auditPlatform(ctx, userID, "sessions_revoked", req.UserId,
		fmt.Sprintf("%d sesi diakhiri paksa", ended))
	return &hajjv1.RevokeSessionsResponse{SessionsEnded: int32(ended)}, nil
}

func (s *PlatformService) ListKycRecords(ctx context.Context, req *hajjv1.ListKycRecordsRequest) (*hajjv1.ListKycRecordsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	status := ""
	limit := int32(50)
	if req != nil {
		status = req.Status
		if req.Limit > 0 {
			limit = req.Limit
		}
	}
	records, err := s.kycRepository.List(ctx, status, limit)
	if err != nil {
		return nil, serviceError("PlatformService.ListKycRecords", err)
	}
	// The list carries no identity numbers, but it is still a list of real
	// people by name and status, read from the platform panel. Recorded once
	// per admin per day rather than once per request: this screen is opened
	// repeatedly while working through a queue, and a row per open would bury
	// the reads that matter.
	s.recordPersonalDataRead(ctx, "/hajj.v1.PlatformService/ListKycRecords")

	result := &hajjv1.ListKycRecordsResponse{Records: make([]*hajjv1.KycRecordSummary, 0, len(records))}
	for _, record := range records {
		// Summaries carry no identity numbers, deliberately: a list that does
		// turns one careless screenshot into a leak of everybody on it.
		result.Records = append(result.Records, kycSummary(record))
	}
	return result, nil
}

// GetKycRecord returns the identity numbers in the clear.
//
// Audit-logged on every read, without exception. Reading somebody's NIK is not
// a neutral act, and a record of who looked is the only thing that makes the
// access reviewable afterwards.
func (s *PlatformService) GetKycRecord(ctx context.Context, req *hajjv1.GetKycRecordRequest) (*hajjv1.GetKycRecordResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.SubjectId) {
		return nil, serviceError("PlatformService.GetKycRecord", apperror.ErrValidation)
	}
	record, err := s.kycRepository.ForSubject(ctx, req.SubjectType, req.SubjectId)
	if err != nil {
		return nil, serviceError("PlatformService.GetKycRecord", err)
	}
	s.auditPlatform(ctx, userID, "kyc_record_read", record.ID,
		"Data identitas "+record.FullName+" dibuka")
	// Both, and deliberately. The audit entry names the person whose identity
	// was opened, which is what an incident review needs; the read ledger keeps
	// the running count per day, which is what shows a pattern.
	s.recordPersonalDataRead(ctx, "/hajj.v1.PlatformService/GetKycRecord")
	return &hajjv1.GetKycRecordResponse{
		Summary: kycSummary(record), Nik: record.NIK, Npwp: record.NPWP,
		Address: record.Address, PlaceOfBirth: record.PlaceOfBirth,
	}, nil
}

func (s *PlatformService) SetKycStatus(ctx context.Context, req *hajjv1.SetKycStatusRequest) (*hajjv1.SetKycStatusResponse, error) {
	userID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.RecordId) {
		return nil, serviceError("PlatformService.SetKycStatus", apperror.ErrValidation)
	}
	if req.Status == "REJECTED" && strings.TrimSpace(req.Reason) == "" {
		// A rejection somebody cannot act on is a rejection that will be
		// resubmitted unchanged.
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("sertakan alasan penolakan agar dapat diperbaiki"))
	}
	if err := s.kycRepository.SetStatus(ctx, req.RecordId, req.Status, userID, req.Reason); err != nil {
		return nil, serviceError("PlatformService.SetKycStatus", err)
	}
	s.auditPlatform(ctx, userID, "kyc_status_set", req.RecordId,
		"Status identitas diubah menjadi "+req.Status+noteSuffix(req.Reason))
	return &hajjv1.SetKycStatusResponse{}, nil
}

func kycSummary(record *repository.KYCRecord) *hajjv1.KycRecordSummary {
	summary := &hajjv1.KycRecordSummary{
		Id: record.ID, OperatorName: record.OperatorName, UserId: record.UserID,
		SubjectType: record.SubjectType, SubjectId: record.SubjectID, FullName: record.FullName,
		Status: record.Status, Source: record.Source, VerifiedBy: record.VerifiedBy,
		RejectionReason: record.RejectionReason, CreatedAt: timestamppb.New(record.CreatedAt),
	}
	if record.VerifiedAt != nil {
		summary.VerifiedAt = timestamppb.New(*record.VerifiedAt)
	}
	return summary
}

// recordPersonalDataRead notes a platform-side read of personal data.
//
// Best effort: a support person must not be shown an error because the
// bookkeeping failed, and a jamaah's record must not become unreadable because
// a log table is full. Failures are reported rather than swallowed.
func (s *PlatformService) recordPersonalDataRead(ctx context.Context, procedure string) {
	if s.personalDataReads == nil {
		return
	}
	if err := s.personalDataReads.Record(ctx, repository.PersonalDataRead{
		ActorUserID: middleware.UserIDFromCtx(ctx), Procedure: procedure,
	}); err != nil {
		sentry.CaptureException(fmt.Errorf("PlatformService.recordPersonalDataRead: %s: %w", procedure, err))
	}
}

func (s *PlatformService) ListPersonalDataReads(ctx context.Context, req *hajjv1.ListPersonalDataReadsRequest) (*hajjv1.ListPersonalDataReadsResponse, error) {
	if _, err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil || !isUUID(req.OperatorId) || s.personalDataReads == nil {
		return nil, serviceError("PlatformService.ListPersonalDataReads", apperror.ErrValidation)
	}
	rows, err := s.personalDataReads.ListForOperator(ctx, req.OperatorId, req.Limit)
	if err != nil {
		return nil, serviceError("PlatformService.ListPersonalDataReads", err)
	}
	response := &hajjv1.ListPersonalDataReadsResponse{Reads: make([]*hajjv1.PersonalDataReadRow, 0, len(rows))}
	for _, row := range rows {
		response.Reads = append(response.Reads, &hajjv1.PersonalDataReadRow{
			Actor: row.Actor, Procedure: row.Procedure, Day: row.Day,
			ReadCount: row.ReadCount, LastAt: row.LastAt, InsideTenantView: row.InsideView,
		})
	}
	return response, nil
}
