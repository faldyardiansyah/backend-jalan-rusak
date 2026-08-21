package utils

import (
	"context"
	"mime/multipart"
	"os"
	"time"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// buat upload file ke Cloudinary dan mengembalikan URL
func UploadCloudinary(fileHeader *multipart.FileHeader) (string, error) {
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
			Folder: "laporan_jalan",
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