package utils

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Helper untuk membuat *multipart.FileHeader dari in-memory buffer
func createTestFileHeader(t *testing.T, fieldName, filename string, content []byte, customContentType ...string) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	partHeader := make(map[string][]string)
	partHeader["Content-Disposition"] = []string{`form-data; name="` + fieldName + `"; filename="` + filename + `"`}
	if len(customContentType) > 0 && customContentType[0] != "" {
		partHeader["Content-Type"] = []string{customContentType[0]}
	} else {
		partHeader["Content-Type"] = []string{"application/octet-stream"}
	}

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("failed to create part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write part content: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	err = req.ParseMultipartForm(int64(len(content) + 4096))
	if err != nil {
		t.Fatalf("failed to parse multipart form: %v", err)
	}

	fileHeaders := req.MultipartForm.File[fieldName]
	if len(fileHeaders) == 0 {
		t.Fatalf("no file header found for field %s", fieldName)
	}

	return fileHeaders[0]
}

func TestValidateImageFile(t *testing.T) {
	// Magic bytes untuk test fixtures
	validJPG := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00")
	validPNG := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4")
	validWEBP := []byte("RIFF\x1a\x00\x00\x00WEBPVP8 \x0e\x00\x00\x00\x30\x01\x00\x9d\x01\x2a\x01\x00\x01\x00\x02\x00\x34\x25")
	plainText := []byte("This is just plain text masquerading as an image.")
	pdfBytes := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF")
	exeBytes := []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xFF\xFF\x00\x00")

	// 1. Valid Images <= 5MB
	t.Run("Valid JPEG Image", func(t *testing.T) {
		fh := createTestFileHeader(t, "file", "photo.jpg", validJPG)
		err := ValidateImageFile(fh)
		if err != nil {
			t.Errorf("expected valid JPG to pass, got error: %v", err)
		}
	})

	t.Run("Valid PNG Image", func(t *testing.T) {
		fh := createTestFileHeader(t, "file", "photo.png", validPNG)
		err := ValidateImageFile(fh)
		if err != nil {
			t.Errorf("expected valid PNG to pass, got error: %v", err)
		}
	})

	t.Run("Valid WEBP Image", func(t *testing.T) {
		fh := createTestFileHeader(t, "file", "photo.webp", validWEBP)
		err := ValidateImageFile(fh)
		if err != nil {
			t.Errorf("expected valid WEBP to pass, got error: %v", err)
		}
	})

	t.Run("Valid Image with Uppercase Extension (.JPG, .PNG)", func(t *testing.T) {
		fhJPG := createTestFileHeader(t, "file", "CAMERA.JPG", validJPG)
		if err := ValidateImageFile(fhJPG); err != nil {
			t.Errorf("expected uppercase .JPG to pass, got error: %v", err)
		}

		fhPNG := createTestFileHeader(t, "file", "SCREENSHOT.PNG", validPNG)
		if err := ValidateImageFile(fhPNG); err != nil {
			t.Errorf("expected uppercase .PNG to pass, got error: %v", err)
		}
	})

	// 2. Image > 5MB
	t.Run("Oversized Image (> 5MB) Rejected", func(t *testing.T) {
		// Buat buffer > 5 MB diawali valid JPG bytes
		oversized := make([]byte, MaxUploadImageSizeBytes+1)
		copy(oversized, validJPG)
		fh := createTestFileHeader(t, "file", "large.jpg", oversized)
		err := ValidateImageFile(fh)
		if err == nil {
			t.Errorf("expected oversized image (>5MB) to be rejected, but got nil")
		} else if !strings.Contains(err.Error(), "melebihi batas maksimal") {
			t.Errorf("expected error message mentioning limit, got: %v", err)
		}
	})

	// 3. Non-image file (disallowed extension)
	t.Run("Disallowed Extension (.pdf, .txt, .exe) Rejected", func(t *testing.T) {
		fhPDF := createTestFileHeader(t, "file", "document.pdf", pdfBytes)
		if err := ValidateImageFile(fhPDF); err == nil {
			t.Errorf("expected .pdf to be rejected, got nil")
		}

		fhTXT := createTestFileHeader(t, "file", "notes.txt", plainText)
		if err := ValidateImageFile(fhTXT); err == nil {
			t.Errorf("expected .txt to be rejected, got nil")
		}

		fhEXE := createTestFileHeader(t, "file", "malware.exe", exeBytes)
		if err := ValidateImageFile(fhEXE); err == nil {
			t.Errorf("expected .exe to be rejected, got nil")
		}
	})

	// 4. Fake image extension (extension image but content non-image)
	t.Run("Fake Extension: Text renamed to .jpg Rejected", func(t *testing.T) {
		fh := createTestFileHeader(t, "file", "fake_image.jpg", plainText)
		err := ValidateImageFile(fh)
		if err == nil {
			t.Errorf("expected fake .jpg with plain text content to be rejected, got nil")
		} else if !strings.Contains(err.Error(), "Tipe file tidak valid") {
			t.Errorf("expected invalid type error, got: %v", err)
		}
	})

	t.Run("Fake Extension: Executable renamed to .png Rejected", func(t *testing.T) {
		fh := createTestFileHeader(t, "file", "fake_image.png", exeBytes)
		err := ValidateImageFile(fh)
		if err == nil {
			t.Errorf("expected fake .png with executable content to be rejected, got nil")
		} else if !strings.Contains(err.Error(), "Tipe file tidak valid") {
			t.Errorf("expected invalid type error, got: %v", err)
		}
	})

	// 5. Client Manipulates Content-Type header
	t.Run("Manipulated Content-Type Header Spoofing Rejected", func(t *testing.T) {
		// Client claims Content-Type is image/jpeg, but actual bytes are text
		fh := createTestFileHeader(t, "file", "spoofed.jpg", plainText, "image/jpeg")
		err := ValidateImageFile(fh)
		if err == nil {
			t.Errorf("expected spoofed Content-Type header to be caught by byte sniffing, got nil")
		} else if !strings.Contains(err.Error(), "Tipe file tidak valid") {
			t.Errorf("expected invalid type error, got: %v", err)
		}
	})

	// 6. Corrupt / Empty File
	t.Run("Empty File (0 bytes) Rejected", func(t *testing.T) {
		fh := createTestFileHeader(t, "file", "empty.jpg", []byte{})
		err := ValidateImageFile(fh)
		if err == nil {
			t.Errorf("expected 0 byte file to be rejected, got nil")
		}
	})

	t.Run("Corrupt Random Binary with .jpg extension Rejected", func(t *testing.T) {
		corrupt := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
		fh := createTestFileHeader(t, "file", "corrupt.jpg", corrupt)
		err := ValidateImageFile(fh)
		if err == nil {
			t.Errorf("expected corrupt bytes to be rejected, got nil")
		}
	})

	// 7. Nil FileHeader
	t.Run("Nil FileHeader Rejected", func(t *testing.T) {
		err := ValidateImageFile(nil)
		if err == nil {
			t.Errorf("expected nil FileHeader to be rejected, got nil")
		}
	})

	// 8. Custom Max Size parameter
	t.Run("Custom Max Size Parameter Enforced", func(t *testing.T) {
		customMax := int64(len(validJPG) - 1)
		fh := createTestFileHeader(t, "file", "custom.jpg", validJPG)
		err := ValidateImageFile(fh, customMax)
		if err == nil {
			t.Errorf("expected file exceeding custom max size to be rejected, got nil")
		}
	})
}
