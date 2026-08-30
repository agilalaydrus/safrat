package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
	"github.com/hajj-saas/api/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ShipmentService is the travel's own delivery work: equipment somebody packs
// and hands over. Separate from FulfilmentService, which talks to suppliers —
// the two look alike from a distance and share nothing operationally.
type ShipmentService struct {
	operatorRepository   *repository.OperatorRepository
	fulfilmentRepository *repository.FulfilmentRepository
	auditRepository      *repository.AuditRepository
	// Refunds when a parcel is declared lost. Composed rather than
	// reimplemented — a second copy of the refund path is the last thing that
	// should ever exist twice.
	orders *OrderService
	// Nil when object storage is unconfigured. Photo proof then simply is not
	// offered, rather than the whole delivery workflow failing to start —
	// recording a handover by name is still worth doing.
	objectStorage *storage.Store
}

func NewShipmentService(operators *repository.OperatorRepository, fulfilments *repository.FulfilmentRepository, audit *repository.AuditRepository, objects *storage.Store, orders *OrderService) *ShipmentService {
	return &ShipmentService{operatorRepository: operators, fulfilmentRepository: fulfilments, auditRepository: audit, objectStorage: objects, orders: orders}
}

// MarkLost closes out a parcel that never arrived, and returns the money.
//
// The order of operations matters. The fulfilment moves first: that update is
// conditional on the parcel still being open, so it is what stops two clicks
// becoming two refunds. Refunding first would leave a window where the money is
// gone and the record still says the parcel is on its way.
func (s *ShipmentService) MarkLost(ctx context.Context, orgID, userID string, req *hajjv1.MarkShipmentLostRequest) (*hajjv1.MarkShipmentLostResponse, error) {
	if req == nil || !isUUID(req.OrderId) || strings.TrimSpace(req.Note) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("sertakan alasan: menyatakan paket hilang melepaskan uang, dan tidak ada yang mengonfirmasinya di luar sistem"))
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ShipmentService.MarkLost", err)
	}

	note := strings.TrimSpace(req.Note)
	moved, err := s.fulfilmentRepository.MarkShipmentLost(ctx, op.ID, req.OrderId, note, userID)
	if err != nil {
		return nil, serviceError("ShipmentService.MarkLost", err)
	}
	if !moved {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("paket ini sudah tidak dalam perjalanan; mungkin sudah diserahkan atau sudah ditandai hilang"))
	}

	_ = s.auditRepository.Write(ctx, op.ID, userID, "shipment_lost", "order", req.OrderId, note)

	response := &hajjv1.MarkShipmentLostResponse{}
	if s.orders != nil {
		// Derived from the order rather than random, so a retry after a network
		// error settles the same refund instead of opening a second one.
		refund, refundErr := s.orders.RefundOrderForOperator(ctx, op.ID, userID, &hajjv1.RefundOrderRequest{
			OrderId:        req.OrderId,
			Reason:         "Paket tidak sampai: " + note,
			IdempotencyKey: "shipment-lost-" + req.OrderId,
		})
		if refundErr != nil {
			// The parcel is already FAILED and the money is not back. Said
			// plainly: somebody has to finish it by hand, and a generic error
			// would hide which half succeeded.
			return nil, connect.NewError(connect.CodeInternal,
				fmt.Errorf("paket sudah ditandai hilang tetapi refund belum berhasil; selesaikan refund order %s secara manual: %w", req.OrderId, refundErr))
		}
		response.Refunded = true
		if refund.Refund != nil {
			response.RefundedIdr = refund.Refund.AmountIdr
		}
	}

	shipment, err := s.read(ctx, op.ID, req.OrderId, "ShipmentService.MarkLost")
	if err != nil {
		return nil, err
	}
	response.Shipment = shipment
	return response, nil
}

// CreateProofUpload hands back a one-shot link for the receipt photo.
func (s *ShipmentService) CreateProofUpload(ctx context.Context, orgID string, req *hajjv1.CreateHandoverProofUploadRequest) (*hajjv1.CreateHandoverProofUploadResponse, error) {
	if req == nil || !isUUID(req.OrderId) || req.SizeBytes <= 0 {
		return nil, serviceError("ShipmentService.CreateProofUpload", apperror.ErrValidation)
	}
	if s.objectStorage == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("penyimpanan berkas belum dikonfigurasi; serah terima tetap dapat dicatat tanpa foto"))
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ShipmentService.CreateProofUpload", err)
	}
	// Read first, so an upload cannot be presigned for an order belonging to
	// somebody else.
	if _, err := s.fulfilmentRepository.GetShipment(ctx, op.ID, req.OrderId); err != nil {
		return nil, serviceError("ShipmentService.CreateProofUpload", err)
	}

	upload, err := s.objectStorage.PresignHandoverUpload(ctx, op.ID, req.OrderId, req.SizeBytes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return &hajjv1.CreateHandoverProofUploadResponse{
		UploadUrl: upload.UploadURL, ObjectKey: upload.ObjectKey, ContentType: upload.ContentType,
	}, nil
}

// GetProofURL returns a short-lived link to a stored photo.
//
// Fetched on demand rather than included in the shipment list: a list carrying
// links to people's doorways is a list that leaks the moment it is logged,
// cached or screenshotted.
func (s *ShipmentService) GetProofURL(ctx context.Context, orgID string, req *hajjv1.GetHandoverProofUrlRequest) (*hajjv1.GetHandoverProofUrlResponse, error) {
	if req == nil || !isUUID(req.OrderId) {
		return nil, serviceError("ShipmentService.GetProofURL", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ShipmentService.GetProofURL", err)
	}
	shipment, err := s.fulfilmentRepository.GetShipment(ctx, op.ID, req.OrderId)
	if err != nil {
		return nil, serviceError("ShipmentService.GetProofURL", err)
	}
	if shipment.HandoverProofKey == "" || s.objectStorage == nil {
		return &hajjv1.GetHandoverProofUrlResponse{}, nil
	}
	url, err := s.objectStorage.PresignHandoverView(ctx, op.ID, req.OrderId, shipment.HandoverProofKey)
	if err != nil {
		return nil, serviceError("ShipmentService.GetProofURL", err)
	}
	return &hajjv1.GetHandoverProofUrlResponse{ViewUrl: url}, nil
}

func (s *ShipmentService) ListShipments(ctx context.Context, orgID string, req *hajjv1.ListShipmentsRequest) (*hajjv1.ListShipmentsResponse, error) {
	if req == nil {
		return nil, serviceError("ShipmentService.ListShipments", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ShipmentService.ListShipments", err)
	}
	shipments, err := s.fulfilmentRepository.ListShipments(ctx, op.ID, req.IncludeDelivered)
	if err != nil {
		return nil, serviceError("ShipmentService.ListShipments", err)
	}
	out := make([]*hajjv1.Shipment, 0, len(shipments))
	for _, shipment := range shipments {
		out = append(out, shipmentMessage(shipment))
	}
	return &hajjv1.ListShipmentsResponse{Shipments: out}, nil
}

func (s *ShipmentService) SaveDestination(ctx context.Context, orgID, userID string, req *hajjv1.SaveShipmentDestinationRequest) (*hajjv1.Shipment, error) {
	if req == nil || !isUUID(req.OrderId) {
		return nil, serviceError("ShipmentService.SaveDestination", apperror.ErrValidation)
	}

	// A posted parcel needs somewhere to go and someone to receive it. Checked
	// here as well as in the database so the person typing gets a sentence
	// rather than a constraint name.
	method := strings.TrimSpace(req.DeliveryMethod)
	address := strings.TrimSpace(req.ShippingAddress)
	recipient := strings.TrimSpace(req.RecipientName)
	if method == "SHIP" && (address == "" || recipient == "") {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("pengiriman butuh nama penerima dan alamat lengkap"))
	}

	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ShipmentService.SaveDestination", err)
	}
	err = s.fulfilmentRepository.SaveShipmentDestination(ctx, op.ID, req.OrderId, repository.Shipment{
		DeliveryMethod: method, RecipientName: recipient,
		RecipientPhone: strings.TrimSpace(req.RecipientPhone), ShippingAddress: address,
	})
	if errors.Is(err, apperror.ErrFailedPrecondition) {
		// The row exists but would not move, which for this update means one
		// thing: it has already been sent.
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("paket sudah berangkat; tujuan tidak dapat diubah lagi"))
	}
	if err != nil {
		return nil, serviceError("ShipmentService.SaveDestination", err)
	}

	_ = s.auditRepository.Write(ctx, op.ID, userID, "shipment_destination_set", "order", req.OrderId,
		"metode "+method)
	return s.read(ctx, op.ID, req.OrderId, "ShipmentService.SaveDestination")
}

func (s *ShipmentService) MarkSent(ctx context.Context, orgID, userID string, req *hajjv1.MarkShipmentSentRequest) (*hajjv1.Shipment, error) {
	if req == nil || !isUUID(req.OrderId) ||
		strings.TrimSpace(req.Courier) == "" || strings.TrimSpace(req.TrackingNumber) == "" {
		return nil, serviceError("ShipmentService.MarkSent", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ShipmentService.MarkSent", err)
	}

	err = s.fulfilmentRepository.MarkShipmentSent(ctx, op.ID, req.OrderId,
		strings.TrimSpace(req.Courier), strings.TrimSpace(req.TrackingNumber))
	if errors.Is(err, apperror.ErrFailedPrecondition) {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("paket ini tidak sedang menunggu pengiriman; mungkin sudah dikirim atau diserahkan"))
	}
	if errors.Is(err, apperror.ErrConflict) || errors.Is(err, apperror.ErrValidation) {
		// The row-level CHECK refusing means the destination was never
		// completed. That is a different fix from a wrong status.
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("lengkapi tujuan pengiriman sebelum menandai terkirim"))
	}
	if err != nil {
		return nil, serviceError("ShipmentService.MarkSent", err)
	}

	_ = s.auditRepository.Write(ctx, op.ID, userID, "shipment_sent", "order", req.OrderId,
		strings.TrimSpace(req.Courier)+" "+strings.TrimSpace(req.TrackingNumber))
	return s.read(ctx, op.ID, req.OrderId, "ShipmentService.MarkSent")
}

func (s *ShipmentService) MarkHandedOver(ctx context.Context, orgID, userID string, req *hajjv1.MarkShipmentHandedOverRequest) (*hajjv1.Shipment, error) {
	if req == nil || !isUUID(req.OrderId) || strings.TrimSpace(req.HandoverRecipient) == "" {
		return nil, serviceError("ShipmentService.MarkHandedOver", apperror.ErrValidation)
	}
	op, err := s.operatorRepository.GetByBetterAuthOrgID(ctx, orgID)
	if err != nil {
		return nil, serviceError("ShipmentService.MarkHandedOver", err)
	}

	// Verified before it is stored. A key recorded for an object that does not
	// exist would read as evidence of something that never happened — worse
	// than no photo, because the row would claim the handover was documented.
	proofKey := strings.TrimSpace(req.HandoverProofKey)
	if proofKey != "" {
		if s.objectStorage == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("penyimpanan berkas belum dikonfigurasi"))
		}
		if err := s.objectStorage.ConfirmHandoverUpload(ctx, op.ID, req.OrderId, proofKey); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.New("foto bukti tidak ditemukan di penyimpanan; ulangi unggahannya"))
		}
	}

	err = s.fulfilmentRepository.MarkShipmentHandedOver(ctx, op.ID, req.OrderId,
		strings.TrimSpace(req.HandoverRecipient), strings.TrimSpace(req.HandoverNote), proofKey, userID)
	if errors.Is(err, apperror.ErrFailedPrecondition) {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("paket ini sudah tercatat diserahkan"))
	}
	if err != nil {
		return nil, serviceError("ShipmentService.MarkHandedOver", err)
	}

	// Audited with the recipient's name, because this is the record a dispute
	// is argued with and the audit trail is where it has to survive.
	_ = s.auditRepository.Write(ctx, op.ID, userID, "shipment_handed_over", "order", req.OrderId,
		"diterima oleh "+strings.TrimSpace(req.HandoverRecipient))
	return s.read(ctx, op.ID, req.OrderId, "ShipmentService.MarkHandedOver")
}

// read returns the row as the database now holds it rather than echoing the
// request back, so the caller sees what was actually stored — including the
// status the write moved it to.
func (s *ShipmentService) read(ctx context.Context, operatorID, orderID, method string) (*hajjv1.Shipment, error) {
	shipment, err := s.fulfilmentRepository.GetShipment(ctx, operatorID, orderID)
	if err != nil {
		return nil, serviceError(method, err)
	}
	return shipmentMessage(shipment), nil
}

func shipmentMessage(shipment *repository.Shipment) *hajjv1.Shipment {
	message := &hajjv1.Shipment{
		OrderId: shipment.OrderID, ReceiptNumber: shipment.ReceiptNumber,
		ProductName: shipment.ProductName, BuyerName: shipment.BuyerName,
		Quantity: shipment.Quantity, TotalPriceIdr: shipment.TotalPriceIDR,
		Status: shipment.Status, DeliveryMethod: shipment.DeliveryMethod,
		RecipientName: shipment.RecipientName, RecipientPhone: shipment.RecipientPhone,
		ShippingAddress: shipment.ShippingAddress, Courier: shipment.Courier,
		TrackingNumber:    shipment.TrackingNumber,
		HandoverRecipient: shipment.HandoverRecipient, HandoverNote: shipment.HandoverNote,
		HasHandoverProof:    shipment.HandoverProofKey != "",
		DestinationEditable: shipment.DestinationEditable(),
	}
	if shipment.PaidAt != nil {
		message.PaidAt = timestamppb.New(*shipment.PaidAt)
	}
	if shipment.SentAt != nil {
		message.SentAt = timestamppb.New(*shipment.SentAt)
	}
	if shipment.DeliveredAt != nil {
		message.DeliveredAt = timestamppb.New(*shipment.DeliveredAt)
	}
	return message
}
