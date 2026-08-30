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

// NewPilgrimSelfDocumentUploadHandler is the pilgrim self-upload
// counterpart to NewDocumentUploadHandler — public, authenticated by
// app_access_code (form field) instead of a staff bearer token, same
// pattern as the rest of the pilgrim-facing surface (ReportLost,
// CreateSOSAlert, ...). Plain net/http for the same multipart-support
// reason as the admin upload handler.
func NewPilgrimSelfDocumentUploadHandler(pilgrimService *service.PilgrimService, uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			http.Error(w, "file too large (max 10MB)", http.StatusBadRequest)
			return
		}
		appAccessCode := r.FormValue("app_access_code")
		docType := r.FormValue("doc_type")
		if strings.TrimSpace(appAccessCode) == "" {
			http.Error(w, "missing app_access_code", http.StatusBadRequest)
			return
		}

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
		safeName := fmt.Sprintf("pilgrim-self-%s%s", uuid.NewString(), ext)
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
		document, err := pilgrimService.CreateDocumentSelf(r.Context(), appAccessCode, docType, fileURL, header.Filename)
		if err != nil {
			_ = os.Remove(filepath.Join(uploadDir, safeName))
			http.Error(w, "invalid upload request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"url": fileURL, "id": document.ID})
	}
}

// NewAgentDocumentUploadHandler is the admin-facing counterpart for
// Agent/Muttawwif KYC documents — bearer-token (staff session) authenticated,
// same as NewDocumentUploadHandler.
func NewAgentDocumentUploadHandler(pool *pgxpool.Pool, agentService *service.AgentService, uploadDir string) http.HandlerFunc {
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
		agentID := r.FormValue("agent_id")
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
		safeName := fmt.Sprintf("%s-%s%s", strings.ReplaceAll(agentID, "-", ""), uuid.NewString(), ext)
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
		document, err := agentService.CreateDocument(staffCtx, authenticatedOrgID, agentID, docType, fileURL, header.Filename)
		if err != nil {
			_ = os.Remove(filepath.Join(uploadDir, safeName))
			http.Error(w, "invalid upload request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"url": fileURL, "id": document.ID})
	}
}

// NewAgentSelfDocumentUploadHandler is the Agent/Muttawwif self-upload
// counterpart — bearer-token (staff session) authenticated, but resolves
// "which agent" from the caller's own identity (CreateDocumentSelf), never
// trusting an agent_id from the form.
func NewAgentSelfDocumentUploadHandler(pool *pgxpool.Pool, agentService *service.AgentService, uploadDir string) http.HandlerFunc {
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
		safeName := fmt.Sprintf("agent-self-%s%s", uuid.NewString(), ext)
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
		document, err := agentService.CreateDocumentSelf(staffCtx, authenticatedOrgID, userID, docType, fileURL, header.Filename)
		if err != nil {
			_ = os.Remove(filepath.Join(uploadDir, safeName))
			http.Error(w, "invalid upload request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"url": fileURL, "id": document.ID})
	}
}
