package upload

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveFile_ValidImage(t *testing.T) {
	cfg := AvatarConfig
	cfg.Directory = t.TempDir()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("test", "value")
	writer.Close()

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := SaveFile(req, "avatar", cfg)
	if err == nil {
		t.Fatal("expected error for no file")
	}

	uploadErr, ok := err.(*UploadError)
	if !ok {
		t.Fatal("expected UploadError")
	}

	if uploadErr.Code != "NO_FILE" {
		t.Errorf("expected NO_FILE, got %s", uploadErr.Code)
	}
}

func TestSaveFile_FileTooLarge(t *testing.T) {
	cfg := AvatarConfig
	cfg.MaxSize = 10
	cfg.Directory = t.TempDir()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("avatar", "large.jpg")
	_, _ = part.Write(bytes.Repeat([]byte("x"), 20))
	writer.Close()

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := SaveFile(req, "avatar", cfg)
	if err == nil {
		t.Fatal("expected error for file too large")
	}

	uploadErr, ok := err.(*UploadError)
	if !ok {
		t.Fatal("expected UploadError")
	}

	if uploadErr.Code != "FILE_TOO_LARGE" {
		t.Errorf("expected FILE_TOO_LARGE, got %s", uploadErr.Code)
	}
}

func TestSaveFile_InvalidExtension(t *testing.T) {
	cfg := AvatarConfig
	cfg.Directory = t.TempDir()
	cfg.AllowedMIME = append(cfg.AllowedMIME, "application/octet-stream")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("avatar", "test.exe")
	_, _ = part.Write([]byte("content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := SaveFile(req, "avatar", cfg)
	if err == nil {
		t.Fatal("expected error for invalid extension")
	}

	uploadErr, ok := err.(*UploadError)
	if !ok {
		t.Fatal("expected UploadError")
	}

	if uploadErr.Code != "INVALID_EXTENSION" {
		t.Errorf("expected INVALID_EXTENSION, got %s", uploadErr.Code)
	}
}

func TestSaveFile_NoFile(t *testing.T) {
	cfg := AvatarConfig
	cfg.Directory = t.TempDir()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := SaveFile(req, "avatar", cfg)
	if err == nil {
		t.Fatal("expected error when no file")
	}

	uploadErr, ok := err.(*UploadError)
	if !ok {
		t.Fatal("expected UploadError")
	}

	if uploadErr.Code != "NO_FILE" {
		t.Errorf("expected NO_FILE, got %s", uploadErr.Code)
	}
}

func TestIsUploadError(t *testing.T) {
	err := &UploadError{Code: "TEST", Message: "test"}
	if !IsUploadError(err) {
		t.Error("expected true for UploadError")
	}

	if IsUploadError(nil) {
		t.Error("expected false for nil")
	}

	if IsUploadError(os.ErrNotExist) {
		t.Error("expected false for regular error")
	}
}

func TestDeleteFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := DeleteFile(testFile); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if FileExists(testFile) {
		t.Error("expected file to be deleted")
	}
}

func TestFileExists(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "exists.txt")
	if err := os.WriteFile(tmpFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	if !FileExists(tmpFile) {
		t.Error("expected file to exist")
	}

	if FileExists("/nonexistent/file.txt") {
		t.Error("expected false for nonexistent file")
	}
}

func TestConfigs(t *testing.T) {
	if AvatarConfig.MaxSize == 0 {
		t.Error("AvatarConfig should have MaxSize set")
	}

	if len(AvatarConfig.AllowedMIME) == 0 {
		t.Error("AvatarConfig should have AllowedMIME set")
	}

	if ImageConfig.MaxSize != 10*1024*1024 {
		t.Errorf("ImageConfig MaxSize should be 10MB, got %d", ImageConfig.MaxSize)
	}

	if DocumentConfig.MaxSize != 25*1024*1024 {
		t.Errorf("DocumentConfig MaxSize should be 25MB, got %d", DocumentConfig.MaxSize)
	}
}
