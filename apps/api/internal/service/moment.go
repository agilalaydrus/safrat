package service

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	"github.com/hajj-saas/api/internal/domain"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MomentService struct {
	operatorRepository *repository.OperatorRepository
	momentRepository   *repository.MomentRepository
	objectStorage      *storage.Store
}

func NewMomentService(operators *repository.OperatorRepository, moments *repository.MomentRepository, objectStorage *storage.Store) *MomentService {
	return &MomentService{operatorRepository: operators, momentRepository: moments, objectStorage: objectStorage}
}

func (s *MomentService) CreateMomentUpload(ctx context.Context, orgID string, req *hajjv1.CreateMomentUploadRequest) (*hajjv1.CreateMomentUploadResponse, error) {
	if req == nil || req.SizeBytes <= 0 {
		return nil, serviceError("MomentService.CreateMomentUpload", apperror.ErrValidation)
	}
	if s.objectStorage == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("penyimpanan berkas belum dikonfigurasi"))
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("MomentService.CreateMomentUpload", err)
	}
	upload, err := s.objectStorage.PresignMomentUpload(ctx, op.ID, req.SizeBytes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return &hajjv1.CreateMomentUploadResponse{UploadUrl: upload.UploadURL, ObjectKey: upload.ObjectKey, ContentType: upload.ContentType}, nil
}

func (s *MomentService) CreateMoment(ctx context.Context, orgID, createdBy string, req *hajjv1.CreateMomentRequest) (*hajjv1.Moment, error) {
	pilgrimID, groupID := strings.TrimSpace(req.GetPilgrimId()), strings.TrimSpace(req.GetGroupId())
	if req == nil || !isUUID(req.SeasonId) || strings.TrimSpace(req.ObjectKey) == "" || (pilgrimID == "") == (groupID == "") {
		return nil, serviceError("MomentService.CreateMoment", apperror.ErrValidation)
	}
	if s.objectStorage == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("penyimpanan berkas belum dikonfigurasi"))
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("MomentService.CreateMoment", err)
	}
	// Verified before it is stored — a key recorded for an object that does
	// not exist would read as a moment that was never actually captured.
	if err := s.objectStorage.ConfirmMomentUpload(ctx, op.ID, req.ObjectKey); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("foto tidak ditemukan di penyimpanan; ulangi unggahannya"))
	}
	moment, err := s.momentRepository.Create(ctx, op.ID, req.SeasonId, pilgrimID, groupID, req.ObjectKey, strings.TrimSpace(req.Caption), createdBy)
	if err != nil {
		return nil, serviceError("MomentService.CreateMoment", err)
	}
	return s.momentMessage(ctx, op.ID, moment), nil
}

func (s *MomentService) DeleteMoment(ctx context.Context, orgID string, req *hajjv1.DeleteMomentRequest) error {
	if req == nil || strings.TrimSpace(req.MomentId) == "" {
		return serviceError("MomentService.DeleteMoment", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return serviceError("MomentService.DeleteMoment", err)
	}
	photoKey, err := s.momentRepository.Delete(ctx, op.ID, req.MomentId)
	if err != nil {
		return serviceError("MomentService.DeleteMoment", err)
	}
	if s.objectStorage != nil && photoKey != "" {
		_ = s.objectStorage.DeleteMomentObject(ctx, op.ID, photoKey)
	}
	return nil
}

func (s *MomentService) ListMoments(ctx context.Context, orgID string, req *hajjv1.ListMomentsRequest) (*hajjv1.ListMomentsResponse, error) {
	if req == nil || !isUUID(req.SeasonId) {
		return nil, serviceError("MomentService.ListMoments", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("MomentService.ListMoments", err)
	}
	moments, err := s.momentRepository.ListForSeason(ctx, op.ID, req.SeasonId)
	if err != nil {
		return nil, serviceError("MomentService.ListMoments", err)
	}
	response := &hajjv1.ListMomentsResponse{}
	for _, m := range moments {
		response.Moments = append(response.Moments, s.momentMessage(ctx, op.ID, m))
	}
	return response, nil
}

// momentMessage resolves a short-lived view link per row rather than storing
// one — a link that outlives its own listing would be a private photo
// reachable by anyone who kept the response around.
func (s *MomentService) momentMessage(ctx context.Context, operatorID string, m *domain.Moment) *hajjv1.Moment {
	msg := &hajjv1.Moment{
		Id: m.ID, SeasonId: m.SeasonID, PilgrimId: m.PilgrimID, PilgrimName: m.PilgrimName,
		GroupId: m.GroupID, GroupName: m.GroupName, Caption: m.Caption, CreatedBy: m.CreatedBy,
		CreatedAt: timestamppb.New(m.CreatedAt),
	}
	if s.objectStorage != nil {
		if url, err := s.objectStorage.PresignMomentView(ctx, operatorID, m.PhotoKey); err == nil {
			msg.PhotoViewUrl = url
		}
	}
	return msg
}
