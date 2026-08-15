package service

import (
	"context"
	"strings"

	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PilgrimAppService struct {
	pilgrimRepository *repository.PilgrimRepository
	productRepository *repository.ProductRepository
}

func NewPilgrimAppService(pilgrims *repository.PilgrimRepository, products *repository.ProductRepository) *PilgrimAppService {
	return &PilgrimAppService{pilgrimRepository: pilgrims, productRepository: products}
}

func (s *PilgrimAppService) GetMyInfo(ctx context.Context, req *hajjv1.PilgrimAppRequest) (*hajjv1.PilgrimAppInfo, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.GetMyInfo", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.GetMyInfo", apperror.ErrNotFound)
	}
	result := &hajjv1.PilgrimAppInfo{Id: info.ID, FullName: info.FullName, PassportNumber: info.PassportNumber, GroupName: info.GroupName, HotelName: info.HotelName, RoomNumber: info.RoomNumber, RequiresWheelchair: info.RequiresWheelchair}
	movements, err := s.pilgrimRepository.ListUpcomingMovements(ctx, info.OperatorID, info.SeasonID)
	if err == nil && len(movements) > 0 {
		m := movements[0]
		result.NextMovement = &hajjv1.Movement{Id: m.ID, OperatorId: m.OperatorID, SeasonId: m.SeasonID, Name: m.Name, Origin: m.Origin, Destination: m.Destination, ScheduledAt: timestamppb.New(m.ScheduledAt), Status: m.Status, CreatedAt: timestamppb.New(m.CreatedAt)}
	}
	return result, nil
}

func (s *PilgrimAppService) ListMySchedule(ctx context.Context, req *hajjv1.PilgrimAppRequest) (*hajjv1.ListMyScheduleResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.ListMySchedule", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMySchedule", apperror.ErrNotFound)
	}
	movements, err := s.pilgrimRepository.ListUpcomingMovements(ctx, info.OperatorID, info.SeasonID)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMySchedule", err)
	}
	result := &hajjv1.ListMyScheduleResponse{Movements: make([]*hajjv1.Movement, 0, len(movements))}
	for _, m := range movements {
		result.Movements = append(result.Movements, &hajjv1.Movement{Id: m.ID, OperatorId: m.OperatorID, SeasonId: m.SeasonID, Name: m.Name, Origin: m.Origin, Destination: m.Destination, ScheduledAt: timestamppb.New(m.ScheduledAt), Status: m.Status, CreatedAt: timestamppb.New(m.CreatedAt)})
	}
	return result, nil
}

func (s *PilgrimAppService) ListMyProducts(ctx context.Context, req *hajjv1.PilgrimAppRequest) (*hajjv1.ListMyProductsResponse, error) {
	if req == nil || strings.TrimSpace(req.AppAccessCode) == "" {
		return nil, serviceError("PilgrimAppService.ListMyProducts", apperror.ErrValidation)
	}
	info, err := s.pilgrimRepository.GetAppInfo(ctx, req.AppAccessCode)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMyProducts", apperror.ErrNotFound)
	}
	products, err := s.productRepository.ListBySeasonID(ctx, info.OperatorID, info.SeasonID)
	if err != nil {
		return nil, serviceError("PilgrimAppService.ListMyProducts", err)
	}
	result := &hajjv1.ListMyProductsResponse{Products: make([]*hajjv1.Product, 0, len(products))}
	for _, p := range products {
		if !p.IsActive {
			continue
		}
		result.Products = append(result.Products, productMessage(p))
	}
	return result, nil
}
