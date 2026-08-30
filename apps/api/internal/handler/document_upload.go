package handler

import (
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

const maxUploadSize = 10 << 20 // 10MB

// NewDocumentUploadHandler is a plain net/http handler, not a Connect RPC —
// Connect's unary protocol has no first-class multipart/form-data support,
// so this stays outside hajjv1connect the same way the Xendit webhook does.
// Registered directly on the mux in main.go, authenticated the same way
// every Connect RPC is (see middleware.ResolveStaffSession).
func NewDocumentUploadHandler(pool *pgxpool.Pool, pilgrimService *service.PilgrimService, uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerTokenFromHeader(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		userID, authenticatedOrgID, _, err := middleware.ResolveStaffSession(r.Context(), pool, token)
		if err != nil {
			http.Error(w, "invalid or expired session", http.StatusUnauthorized)
			return
		}

		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			http.Error(w, "file too large (max 10MB)", http.StatusBadRequest)
			return
		}

		pilgrimID := r.FormValue("pilgrim_id")
		docType := r.FormValue("doc_type")

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		ext := filepath.Ext(header.Filename)
		if len(ext) > 10 {
			ext = ""
		}
		safeName := fmt.Sprintf("%s-%s%s", strings.ReplaceAll(pilgrimID, "-", ""), uuid.NewString(), ext)
		if err := os.MkdirAll(uploadDir, 0o755); err != nil {
			http.Error(w, "failed to prepare upload directory", http.StatusInternalServerError)
			return
		}
		dst, err := os.Create(filepath.Join(uploadDir, safeName))
		if err != nil {
			http.Error(w, "failed to save file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, "failed to save file", http.StatusInternalServerError)
			return
		}

		fileURL := "/uploads/documents/" + safeName
		staffCtx := middleware.ContextWithIdentity(r.Context(), userID, authenticatedOrgID)
		document, err := pilgrimService.CreateDocument(staffCtx, authenticatedOrgID, pilgrimID, docType, fileURL, header.Filename)
		if err != nil {
			_ = os.Remove(filepath.Join(uploadDir, safeName))
			http.Error(w, "invalid upload request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"url": fileURL, "id": document.ID})
	}
}

func bearerTokenFromHeader(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", fmt.Errorf("missing bearer token")
	}
	return parts[1], nil
}
