package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRefundPayoutProofUploadHandler(pool *pgxpool.Pool, payouts *service.RefundPayoutService, uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerTokenFromHeader(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		userID, orgID, role, err := middleware.ResolveStaffSessionRole(r.Context(), pool, token)
		if err != nil {
			http.Error(w, "invalid or expired session", http.StatusUnauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1<<20)
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			http.Error(w, "file too large (max 10MB)", http.StatusBadRequest)
			return
		}
		requestID := strings.TrimSpace(r.FormValue("request_id"))
		if requestID == "" {
			http.Error(w, "missing request_id", http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}
		defer file.Close()
		head := make([]byte, 512)
		n, err := io.ReadFull(file, head)
		if err != nil && err != io.ErrUnexpectedEOF {
			http.Error(w, "invalid file", http.StatusBadRequest)
			return
		}
		head = head[:n]
		mime := http.DetectContentType(head)
		extensions := map[string]string{"application/pdf": ".pdf", "image/jpeg": ".jpg", "image/png": ".png"}
		ext, ok := extensions[mime]
		if !ok {
			http.Error(w, "proof must be PDF, JPG, or PNG", http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(uploadDir, 0o755); err != nil {
			http.Error(w, "failed to prepare upload directory", http.StatusInternalServerError)
			return
		}
		name := fmt.Sprintf("refund-payout-%s%s", uuid.NewString(), ext)
		path := filepath.Join(uploadDir, name)
		dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
		if err != nil {
			http.Error(w, "failed to save proof", http.StatusInternalServerError)
			return
		}
		_, copyErr := io.Copy(dst, io.MultiReader(bytes.NewReader(head), file))
		closeErr := dst.Close()
		if copyErr != nil || closeErr != nil {
			_ = os.Remove(path)
			http.Error(w, "failed to save proof", http.StatusInternalServerError)
			return
		}
		url := "/uploads/documents/" + name
		if _, err := payouts.AttachCashProof(r.Context(), orgID, userID, role, requestID, url); err != nil {
			_ = os.Remove(path)
			http.Error(w, "invalid payout proof request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"url": url})
	}
}
