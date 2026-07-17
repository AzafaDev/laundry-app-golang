package storage

import (
	"context"
	"fmt"
	"mime/multipart"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
)

type Client struct {
	cld *cloudinary.Cloudinary
}

func NewClient(cloudinaryURL string) (*Client, error) {
	cld, err := cloudinary.NewFromURL(cloudinaryURL)
	if err != nil {
		return nil, fmt.Errorf("error in creating cloudinary client: %w", err)
	}

	return &Client{cld: cld}, nil
}

func (c *Client) UploadAvatar(ctx context.Context, file multipart.File, customerID string) (string, error) {
	overwrite := true

	result, err := c.cld.Upload.Upload(ctx, file, uploader.UploadParams{
		PublicID:  customerID,
		Folder:    "customers/avatars",
		Overwrite: &overwrite,
	})
	if err != nil {
		return "", fmt.Errorf("error in uploading avatar: %w", err)
	}
	if result.Error.Message != "" {
		return "", fmt.Errorf("error in uploading avatar: %s", result.Error.Message)
	}

	return result.SecureURL, nil
}

func (c *Client) UploadComplaintPhoto(ctx context.Context, file multipart.File, orderID string) (string, error) {
	result, err := c.cld.Upload.Upload(ctx, file, uploader.UploadParams{
		PublicID: fmt.Sprintf("%s-%s", orderID, uuid.NewString()),
		Folder:   "orders/complaints",
	})
	if err != nil {
		return "", fmt.Errorf("error in uploading complaint photo: %w", err)
	}
	if result.Error.Message != "" {
		return "", fmt.Errorf("error in uploading complaint photo: %s", result.Error.Message)
	}

	return result.SecureURL, nil
}
