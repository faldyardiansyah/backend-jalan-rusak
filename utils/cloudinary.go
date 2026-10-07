package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

const (
	// MaxUploadImageSizeBytes adalah batas maksimal ukuran file upload gambar (5 MB)
	MaxUploadImageSizeBytes = 5 * 1024 * 1024
)

var allowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

var allowedImageMIMEs = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// ValidateImageFile memvalidasi ukuran file, whitelist ekstensi, dan MIME type berbasis byte sniffing.
func ValidateImageFile(fileHeader *multipart.FileHeader, maxSizeBytes ...int64) error {
	if fileHeader == nil {
		return errors.New("File tidak ditemukan")
	}

	maxSize := int64(MaxUploadImageSizeBytes)
	if len(maxSizeBytes) > 0 && maxSizeBytes[0] > 0 {
		maxSize = maxSizeBytes[0]
	}

	if fileHeader.Size == 0 {
		return errors.New("File tidak boleh kosong")
	}

	if fileHeader.Size > maxSize {
		return fmt.Errorf("Ukuran file melebihi batas maksimal (%d MB)", maxSize/(1024*1024))
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedImageExtensions[ext] {
		return errors.New("Ekstensi file tidak didukung. Format yang diizinkan: JPG, PNG, WEBP")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return errors.New("Gagal membaca file")
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return errors.New("Gagal membaca konten file")
	}
	if n == 0 {
		return errors.New("File kosong atau tidak dapat dibaca")
	}

	detectedMIME := http.DetectContentType(buffer[:n])
	detectedMIME = strings.Split(detectedMIME, ";")[0]
	detectedMIME = strings.TrimSpace(detectedMIME)

	if !allowedImageMIMEs[detectedMIME] {
		return errors.New("Tipe file tidak valid. Format yang didukung: image/jpeg, image/png, image/webp")
	}

	return nil
}

// buat upload file ke Cloudinary dan mengembalikan URL
func UploadCloudinary(fileHeader *multipart.FileHeader) (string, error) {
	return UploadCloudinaryWithFolder(fileHeader, "laporan_jalan")
}

// UploadAvatarCloudinary mengunggah file foto avatar ke Cloudinary dalam folder profile_avatars
func UploadAvatarCloudinary(fileHeader *multipart.FileHeader) (string, error) {
	return UploadCloudinaryWithFolder(fileHeader, "profile_avatars")
}

// UploadCloudinaryWithFolder mengunggah file ke Cloudinary pada folder spesifik
func UploadCloudinaryWithFolder(fileHeader *multipart.FileHeader, folder string) (string, error) {
	if err := ValidateImageFile(fileHeader); err != nil {
		return "", err
	}
	cloudName := os.Getenv("CLOUDINARY_CLOUD_NAME")
	apiKey := os.Getenv("CLOUDINARY_API_KEY")
	apiSecret := os.Getenv("CLOUDINARY_API_SECRET")

	// Buat koneksi Cloudinary
	cld, err := cloudinary.NewFromParams(
		cloudName,
		apiKey,
		apiSecret,
	)
	if err != nil {
		return "", err
	}

	// Buka file
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}

	defer file.Close()

	// Buat context dengan timeout 10 detik
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	// Upload ke Cloudinary
	res, err := cld.Upload.Upload(
		ctx,
		file,
		uploader.UploadParams{
			Folder: folder,
		},
	)
	if err != nil {
		return "", err
	}

	// Mengembalikan URL HTTPS dari Cloudinary
	return res.SecureURL, nil
}

// ini itu buat ngedelete
func DeleteCloudinary(fileURL string) error {
	cloudName := os.Getenv("CLOUDINARY_CLOUD_NAME")
	apiKey := os.Getenv("CLOUDINARY_API_KEY")
	apiSecret := os.Getenv("CLOUDINARY_API_SECRET")

	cld, err := cloudinary.NewFromParams(
		cloudName,
		apiKey,
		apiSecret,
	)
	if err != nil {
		return err
	}

	publicID := ExtractPublicID(fileURL)
	if publicID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ini buat manggil api destroy
	_, err = cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID: publicID,
	})

	return err
}

func ExtractPublicID(url string) string {
	parts := strings.Split(url, "/upload/")
	if len(parts) < 2 {
		return ""
	}

	fullPath := parts[1]

	// ini hapus bagian versi yang kaya v1233
	slashIndex := strings.Index(fullPath, "/")
	if slashIndex != -1 {
		potentialPath := fullPath[slashIndex+1:]

		// hapus ekstentsi file
		dotIndex := strings.LastIndex(potentialPath, ".")
		if dotIndex != -1 {
			potentialPath = potentialPath[:dotIndex]
		}

		return potentialPath
	}

	return ""
}