package upload

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/uuid"
)

type Config struct {
	AllowedMIME []string
	AllowedExt  []string
	MaxSize     int64
	Directory   string
}

var (
	AvatarConfig = Config{
		AllowedMIME: []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
		AllowedExt:  []string{".jpg", ".jpeg", ".png", ".webp", ".gif"},
		MaxSize:     5 * 1024 * 1024, // 5MB
		Directory:   "avatars",
	}

	ImageConfig = Config{
		AllowedMIME: []string{"image/jpeg", "image/png", "image/webp"},
		AllowedExt:  []string{".jpg", ".jpeg", ".png", ".webp"},
		MaxSize:     10 * 1024 * 1024, // 10MB
		Directory:   "images",
	}

	DocumentConfig = Config{
		AllowedMIME: []string{"application/pdf"},
		AllowedExt:  []string{".pdf"},
		MaxSize:     25 * 1024 * 1024, // 25MB
		Directory:   "documents",
	}
)

type Result struct {
	Path     string
	Filename string
	Size     int64
	MIMEType string
	URL      string
}

type UploadError struct {
	Code    string
	Message string
}

func (e *UploadError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func IsUploadError(err error) bool {
	_, ok := err.(*UploadError)
	return ok
}

func SaveFile(r *http.Request, fieldName string, cfg Config) (*Result, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return nil, &UploadError{Code: "NO_FILE", Message: "Nenhum arquivo enviado"}
	}
	defer file.Close()

	if header.Size > cfg.MaxSize {
		return nil, &UploadError{
			Code:    "FILE_TOO_LARGE",
			Message: fmt.Sprintf("Arquivo excede o limite de %dMB", cfg.MaxSize/1024/1024),
		}
	}

	contentType := header.Header.Get("Content-Type")
	if !isAllowedMIME(contentType, cfg.AllowedMIME) {
		return nil, &UploadError{
			Code:    "INVALID_TYPE",
			Message: fmt.Sprintf("Tipo de arquivo não permitido: %s", contentType),
		}
	}

	ext := filepath.Ext(header.Filename)
	if !isAllowedExt(ext, cfg.AllowedExt) {
		return nil, &UploadError{
			Code:    "INVALID_EXTENSION",
			Message: fmt.Sprintf("Extensão não permitida: %s", ext),
		}
	}

	if err := os.MkdirAll(cfg.Directory, 0755); err != nil {
		return nil, &UploadError{
			Code:    "DIRECTORY_ERROR",
			Message: "Falha ao criar diretório de upload",
		}
	}

	filename := generateFilename(ext)
	dstPath := filepath.Join(cfg.Directory, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		return nil, &UploadError{
			Code:    "CREATE_ERROR",
			Message: "Falha ao criar arquivo",
		}
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(dstPath)
		return nil, &UploadError{
			Code:    "WRITE_ERROR",
			Message: "Falha ao salvar arquivo",
		}
	}

	url := fmt.Sprintf("/storage/%s/%s", cfg.Directory, filename)

	return &Result{
		Path:     dstPath,
		Filename: filename,
		Size:     written,
		MIMEType: contentType,
		URL:      url,
	}, nil
}

func isAllowedMIME(mime string, allowed []string) bool {
	return slices.Contains(allowed, mime)
}

func isAllowedExt(ext string, allowed []string) bool {
	return slices.Contains(allowed, ext)
}

func generateFilename(ext string) string {
	timestamp := time.Now().Unix()
	unique := uuid.New().String()[:8]
	return fmt.Sprintf("%d_%s%s", timestamp, unique, ext)
}

func DeleteFile(path string) error {
	return os.Remove(path)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
