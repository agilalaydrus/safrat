package service

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// downloadURLSigner is the one thing this service needs from storage: a
// time-limited link to an already-finished file. Building the file itself is
// the worker's job, not a request handler's — an export can take real time,
// and nothing here should hold a connection open for it.
type downloadURLSigner interface {
	PresignDataExportView(ctx context.Context, operatorID, objectKey string) (string, error)
}

type DataExportService struct {
	operatorRepository *repository.OperatorRepository
	exportRepository   *repository.DataExportRepository
	storage            downloadURLSigner
}

func NewDataExportService(operators *repository.OperatorRepository, exports *repository.DataExportRepository, storage downloadURLSigner) *DataExportService {
	return &DataExportService{operatorRepository: operators, exportRepository: exports, storage: storage}
}

func (s *DataExportService) RequestDataExport(ctx context.Context, orgID, userID string, req *hajjv1.RequestDataExportRequest) (*hajjv1.DataExportRow, error) {
	if req == nil || len(strings.TrimSpace(req.IdempotencyKey)) < 8 {
		return nil, serviceError("DataExportService.RequestDataExport", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("DataExportService.RequestDataExport", err)
	}
	row, err := s.exportRepository.Request(ctx, op.ID, userID, strings.TrimSpace(req.IdempotencyKey))
	if err != nil {
		return nil, serviceError("DataExportService.RequestDataExport", err)
	}
	return dataExportMessage(row), nil
}

func (s *DataExportService) ListDataExports(ctx context.Context, orgID string, req *hajjv1.ListDataExportsRequest) (*hajjv1.ListDataExportsResponse, error) {
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("DataExportService.ListDataExports", err)
	}
	limit := int32(20)
	if req != nil && req.Limit > 0 {
		limit = req.Limit
	}
	rows, err := s.exportRepository.ListForOperator(ctx, op.ID, limit)
	if err != nil {
		return nil, serviceError("DataExportService.ListDataExports", err)
	}
	response := &hajjv1.ListDataExportsResponse{}
	for _, row := range rows {
		response.Exports = append(response.Exports, dataExportMessage(row))
	}
	return response, nil
}

func (s *DataExportService) GetDataExportDownloadUrl(ctx context.Context, orgID string, req *hajjv1.GetDataExportDownloadUrlRequest) (*hajjv1.GetDataExportDownloadUrlResponse, error) {
	if req == nil || strings.TrimSpace(req.ExportId) == "" {
		return nil, serviceError("DataExportService.GetDataExportDownloadUrl", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("DataExportService.GetDataExportDownloadUrl", err)
	}
	row, err := s.exportRepository.Get(ctx, op.ID, req.ExportId)
	if errors.Is(err, apperror.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("ekspor tidak ditemukan"))
	}
	if err != nil {
		return nil, serviceError("DataExportService.GetDataExportDownloadUrl", err)
	}
	if row.Status != "READY" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("ekspor belum siap diunduh"))
	}
	if s.storage == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("penyimpanan berkas tidak aktif di server ini"))
	}
	url, err := s.storage.PresignDataExportView(ctx, op.ID, row.ObjectKey)
	if err != nil {
		return nil, serviceError("DataExportService.GetDataExportDownloadUrl", err)
	}
	response := &hajjv1.GetDataExportDownloadUrlResponse{Url: url}
	if row.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(*row.ExpiresAt)
	}
	return response, nil
}

func dataExportMessage(row repository.DataExportRow) *hajjv1.DataExportRow {
	message := &hajjv1.DataExportRow{
		Id: row.ID, Status: row.Status, SizeBytes: row.SizeBytes, Error: row.Error,
		RequestedAt: timestamppb.New(row.RequestedAt),
	}
	if row.CompletedAt != nil {
		message.CompletedAt = timestamppb.New(*row.CompletedAt)
	}
	if row.ExpiresAt != nil {
		message.ExpiresAt = timestamppb.New(*row.ExpiresAt)
	}
	return message
}
