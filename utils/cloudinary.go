package utils

import (
	"context"
	"mime/multipart"
	"os"
	"time"

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