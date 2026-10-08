package handler

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"igb-leads-go/authctx"
	"igb-leads-go/config"
	"igb-leads-go/model"
	"igb-leads-go/repository"
)

// UploadHandler mirrors UploadController (local disk storage).
type UploadHandler struct {
	uploads   *repository.UploadRepository
	uploadDir string
	apiURL    string
}

func NewUploadHandler(cfg *config.Config, uploads *repository.UploadRepository) *UploadHandler {
	return &UploadHandler{uploads: uploads, uploadDir: cfg.UploadDir, apiURL: cfg.APIURL}
}

// allowedImage mirrors the multer fileFilter regex.
var allowedImage = regexp.MustCompile(`jpeg|jpg|png|gif|webp`)

const maxImageBytes = 5 * 1024 * 1024

func (h *UploadHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		WriteError(w, http.StatusBadRequest, "Nenhum arquivo enviado")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		log.Printf("Erro no upload: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	files := r.MultipartForm.File["image"]
	if len(files) == 0 {
		WriteError(w, http.StatusBadRequest, "Nenhum arquivo enviado")
		return
	}
	fh := files[0]
	ext := filepath.Ext(fh.Filename)
	if !allowedImage.MatchString(strings.ToLower(ext)) ||
		!allowedImage.MatchString(strings.ToLower(fh.Header.Get("Content-Type"))) {
		// Multer filter errors surface as 500 through the error middleware.
		WriteError(w, http.StatusInternalServerError, "Apenas imagens são permitidas (jpeg, jpg, png, gif, webp)")
		return
	}
	src, err := fh.Open()
	if err != nil {
		log.Printf("Erro no upload: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	defer src.Close()
	body, err := io.ReadAll(io.LimitReader(src, maxImageBytes+1))
	if err != nil {
		log.Printf("Erro no upload: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	if len(body) > maxImageBytes {
		// Multer's LIMIT_FILE_SIZE message, surfaced as 500.
		WriteError(w, http.StatusInternalServerError, "File too large")
		return
	}
	name := uuid.NewString() + ext
	dir := filepath.Join(h.uploadDir, "campaigns")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("Erro no upload: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
		log.Printf("Erro no upload: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	up := &model.Upload{
		ID: uuid.NewString(), FileName: name, OriginalName: fh.Filename,
		FilePath: "/uploads/campaigns/" + name, FileSize: len(body),
		MimeType: fh.Header.Get("Content-Type"), UploadedBy: authctx.UserID(r),
	}
	if err := h.uploads.Create(r.Context(), up); err != nil {
		os.Remove(filepath.Join(dir, name))
		log.Printf("Erro no upload: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Upload realizado com sucesso",
		"file": map[string]any{
			"id": up.ID, "url": h.apiURL + up.FilePath, "original_name": up.OriginalName,
		},
	})
}

func (h *UploadHandler) Delete(w http.ResponseWriter, r *http.Request) {
	up, err := h.uploads.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, "Arquivo não encontrado")
		return
	}
	if authctx.UserRole(r) != "admin" && up.UploadedBy != authctx.UserID(r) {
		WriteError(w, http.StatusForbidden, "Você só pode remover seus próprios arquivos")
		return
	}
	os.Remove(filepath.Join(h.uploadDir, "campaigns", up.FileName))
	if err := h.uploads.Delete(r.Context(), up.ID); err != nil {
		log.Printf("Erro ao deletar arquivo: %v", err)
		WriteError(w, http.StatusInternalServerError, "Erro interno do servidor")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "Arquivo removido com sucesso"})
}
