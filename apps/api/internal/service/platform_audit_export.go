package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/hajj-saas/api/internal/apperror"
	appcrypto "github.com/hajj-saas/api/internal/crypto"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/repository"
)

// ExportAuditTrail is C4 (TUGAS-PANEL-SAAS.md): the same trail ListAuditTrail
// shows, unbounded and as CSV, closed out with a manifest an auditor can use
// to prove the file they were handed is the exact one this platform
// produced — no more, no less.
//
// The CSV is streamed as raw bytes rather than typed row messages
// (contrast ProfitLossService.StreamProfitLossExport) specifically so the
// bytes hashed for the manifest and the bytes the browser saves to disk are
// provably identical: the client never re-encodes anything, it only
// concatenates what it received.
func (s *PlatformService) ExportAuditTrail(ctx context.Context, req *hajjv1.ExportAuditTrailRequest, stream *connect.ServerStream[hajjv1.AuditExportChunk]) error {
	adminUserID, err := s.requirePlatformAdmin(ctx)
	if err != nil {
		return err
	}
	if s.auditSigner == nil {
		// FailedPrecondition, not the generic Internal bucket serviceError
		// would otherwise map an unrecognised error to: this is a
		// configuration gap an admin can act on, not a bug to page anyone
		// about — same reasoning as OrderService.CreateOrder's missing-
		// Xendit-key case.
		return serviceError("PlatformService.ExportAuditTrail",
			fmt.Errorf("%w: %s", apperror.ErrFailedPrecondition, appcrypto.ErrNoSigningKey))
	}

	filter := repository.AuditFilter{Category: repository.AuditCategoryAll}
	if req != nil {
		filter.OperatorID = strings.TrimSpace(req.OperatorId)
		filter.Actor = strings.TrimSpace(req.Actor)
		if req.Category != "" {
			filter.Category = req.Category
		}
		if req.Days > 0 {
			since := time.Now().AddDate(0, 0, -int(req.Days))
			filter.Since = &since
		}
	}
	if filter.OperatorID != "" && !isUUID(filter.OperatorID) {
		return serviceError("PlatformService.ExportAuditTrail", apperror.ErrValidation)
	}

	hasher := sha256.New()
	sendChunk := func(p []byte) error {
		if len(p) == 0 {
			return nil
		}
		hasher.Write(p)
		chunk := make([]byte, len(p))
		copy(chunk, p)
		return stream.Send(&hajjv1.AuditExportChunk{Payload: &hajjv1.AuditExportChunk_CsvChunk{CsvChunk: chunk}})
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"waktu", "aktor", "id_aktor", "tindakan", "jenis_objek", "id_objek", "id_travel", "nama_travel", "pesan"}); err != nil {
		return serviceError("PlatformService.ExportAuditTrail", err)
	}
	writer.Flush()
	if err := sendChunk(buf.Bytes()); err != nil {
		return serviceError("PlatformService.ExportAuditTrail", err)
	}
	buf.Reset()

	rowCount := 0
	streamErr := s.platformRepository.StreamAuditTrail(ctx, filter, func(entry repository.AuditEntry) error {
		row := []string{
			entry.At.UTC().Format(time.RFC3339), entry.Actor, entry.ActorID, entry.Action,
			entry.EntityType, entry.EntityID, entry.OperatorID, entry.Operator, entry.Message,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
		if err := sendChunk(buf.Bytes()); err != nil {
			return err
		}
		buf.Reset()
		rowCount++
		return nil
	})
	if streamErr != nil {
		return serviceError("PlatformService.ExportAuditTrail", streamErr)
	}

	sum := hex.EncodeToString(hasher.Sum(nil))
	signature, err := s.auditSigner.Sign([]byte(sum))
	if err != nil {
		return serviceError("PlatformService.ExportAuditTrail", err)
	}
	if err := stream.Send(&hajjv1.AuditExportChunk{Payload: &hajjv1.AuditExportChunk_Manifest{Manifest: &hajjv1.AuditExportManifest{
		Sha256: sum, SignedAt: time.Now().UTC().Format(time.RFC3339),
		KeyFingerprint: s.auditSigner.Fingerprint(), HmacSha256: signature,
	}}}); err != nil {
		return serviceError("PlatformService.ExportAuditTrail", err)
	}

	// Exporting the whole trail is itself sensitive — same reasoning as C3
	// logging personal-data reads. Written after the stream finishes, not
	// before, so a client that disconnects mid-export is not credited with
	// one it never actually received.
	_ = s.auditRepository.Write(ctx, "", adminUserID, "audit_trail_exported", "audit_export", "",
		"kategori="+filter.Category+" baris="+strconv.Itoa(rowCount))
	return nil
}
