package controllers

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"meerkat/config"
	apperrors "meerkat/errors"
	"meerkat/httputil"
	"meerkat/idutil"
	"meerkat/logger"
	"meerkat/models"
	"meerkat/photofile"
	"meerkat/services"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gen2brain/heic"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/image/draw"
	"gorm.io/gorm"
)

func GetProfilePicture(c *gin.Context, cfg *config.Config) {
	idParam := c.Param("id")
	contactID, err := idutil.Parse(idParam)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrValidation("Invalid contact ID"))
		return
	}
	var contact models.Contact
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	// Find the contact in the database
	if err := db.Where("user_id = ?", userID).First(&contact, contactID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			apperrors.AbortWithError(c, apperrors.ErrNotFound("Contact"))
			return
		}
		apperrors.AbortWithError(c, apperrors.ErrDatabase("find").WithError(err))
		return
	}

	// Check if thumbnail is requested via query parameter
	wantsThumbnail := c.Query("thumbnail") == "true"

	if wantsThumbnail {
		serveThumbnail(c, cfg, contact)
		return
	}

	// Serve full photo from file using validated config path
	if contact.Photo == "" {
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Photo"))
		return
	}

	// Prevent path traversal attacks
	if strings.Contains(contact.Photo, "..") || filepath.IsAbs(contact.Photo) {
		apperrors.AbortWithError(c, apperrors.ErrValidation("Invalid photo path"))
		return
	}

	filePath := filepath.Join(cfg.ProfilePhotoDir, contact.Photo)
	logger.FromContext(c).Debug().Str("file_path", filePath).Msg("Serving profile picture")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Image"))
		return
	}

	c.File(filePath)
}

// serveThumbnail sends the thumbnail file with an ETag and a long private cache: the URL
// carries the file's unique name (see models.ThumbnailURL), so a new picture is a new URL.
// A contact whose thumbnail was not exported yet still gets the legacy database copy.
func serveThumbnail(c *gin.Context, cfg *config.Config, contact models.Contact) {
	name := contact.PhotoThumbnailFile
	if name == "" {
		serveLegacyThumbnail(c, contact.LegacyThumbnail)
		return
	}
	if strings.Contains(name, "..") || filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		apperrors.AbortWithError(c, apperrors.ErrValidation("Invalid thumbnail path"))
		return
	}
	filePath := filepath.Join(cfg.ProfilePhotoDir, name)
	file, err := os.Open(filePath) //nolint:gosec // name checked above, directory from config
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Thumbnail"))
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Thumbnail"))
		return
	}
	// The name is unique per picture, size and mtime guard against a file replaced in place.
	etag := fmt.Sprintf(`"%s-%x-%x"`, strings.TrimSuffix(name, ".jpg"), info.Size(), info.ModTime().UnixNano())
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.Header("Content-Type", "image/jpeg")
	http.ServeContent(c.Writer, c.Request, "", info.ModTime(), file)
}

func serveLegacyThumbnail(c *gin.Context, legacy string) {
	// Format: data:image/jpeg;base64,<data>
	if !strings.HasPrefix(legacy, "data:") {
		apperrors.AbortWithError(c, apperrors.ErrNotFound("Thumbnail"))
		return
	}
	parts := strings.SplitN(legacy, ",", 2)
	if len(parts) != 2 {
		apperrors.AbortWithError(c, apperrors.ErrInternal("Invalid thumbnail format"))
		return
	}
	imageData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrInternal("Failed to decode thumbnail").WithError(err))
		return
	}
	c.Header("Cache-Control", "private, no-cache")
	c.Data(http.StatusOK, "image/jpeg", imageData)
}

func AddPhotoToContact(c *gin.Context, cfg *config.Config) {
	// Check if demo mode is enabled - photo uploads are disabled in demo
	if os.Getenv("DEMO_MODE") == "true" {
		apperrors.AbortWithError(c, apperrors.ErrForbidden("Photo uploads are disabled in demo mode"))
		return
	}

	idParam := c.Param("id")
	contactID, err := idutil.Parse(idParam)
	if err != nil {
		apperrors.AbortWithError(c, apperrors.ErrValidation("Invalid contact ID"))
		return
	}
	var contact models.Contact
	db := c.MustGet("db").(*gorm.DB)

	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	// Find the contact in the database
	if err := db.Where("user_id = ?", userID).First(&contact, contactID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			apperrors.AbortWithError(c, apperrors.ErrNotFound("Contact"))
			return
		}
		apperrors.AbortWithError(c, apperrors.ErrDatabase("find").WithError(err))
		return
	}

	// Check if there's an uploaded file
	file, err := c.FormFile("photo")
	if err == nil {
		// Validate file size (10MB limit to prevent DoS)
		const maxFileSize = 10 * 1024 * 1024 // 10MB
		if file.Size > maxFileSize {
			apperrors.AbortWithError(c, apperrors.ErrValidation("File too large. Maximum size is 10MB"))
			return
		}

		// Use validated upload directory from config
		if err := photofile.EnsureDir(cfg.ProfilePhotoDir); err != nil {
			apperrors.AbortWithError(c, apperrors.ErrInternal("Failed to create upload directory").WithError(err))
			return
		}
		photoPath, thumbnailPath, err := processAndSavePhoto(file, cfg.ProfilePhotoDir)
		if errors.Is(err, errInvalidPhoto) {
			apperrors.AbortWithError(c, apperrors.ErrInvalidInput("photo", "must be a valid JPEG, PNG or HEIC image of reasonable size").WithError(err))
			return
		}
		if err != nil {
			apperrors.AbortWithError(c, apperrors.ErrInternal("Failed to process photo").WithError(err))
			return
		}
		oldPhoto := contact.Photo
		contact.Photo = photoPath
		oldThumbnail := contact.PhotoThumbnailFile
		contact.PhotoThumbnailFile = thumbnailPath
		contact.LegacyThumbnail = "" // the old picture's copy must not outlive it

		// Save the updated contact
		if err := db.Save(&contact).Error; err != nil {
			// Clean up newly saved file since DB update failed
			os.Remove(filepath.Join(cfg.ProfilePhotoDir, photoPath))
			os.Remove(filepath.Join(cfg.ProfilePhotoDir, thumbnailPath))
			apperrors.AbortWithError(c, apperrors.ErrDatabase("update").WithError(err))
			return
		}

		// Delete old photo file after db update
		services.RemoveContactPhotoFiles(cfg.ProfilePhotoDir, models.Contact{Photo: oldPhoto, PhotoThumbnailFile: oldThumbnail})

		c.JSON(http.StatusOK, contact)
		return
	}

	// Save the updated contact (no new photo uploaded)
	if err := db.Save(&contact).Error; err != nil {
		apperrors.AbortWithError(c, apperrors.ErrDatabase("update").WithError(err))
		return
	}

	c.JSON(http.StatusOK, contact)
}

// errInvalidPhoto marks upload problems the user can fix (wrong format, corrupt or oversized image).
var errInvalidPhoto = errors.New("invalid photo")

// processAndSavePhoto processes an uploaded photo and returns:
// - photoPath: filename of the saved full-size photo
// - thumbnailPath: filename of the saved 48px thumbnail
func processAndSavePhoto(file *multipart.FileHeader, uploadDir string) (string, string, error) {
	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return "", "", err
	}
	defer src.Close()

	// Detect the image format
	buf := make([]byte, 512)
	n, err := io.ReadFull(src, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", "", err
	}
	buf = buf[:n]
	contentType := http.DetectContentType(buf)

	// Check for HEIC/HEIF format (http.DetectContentType doesn't recognize it)
	// HEIC files have "ftyp" followed by "heic", "heix", "hevc", "hevx", or "mif1" at byte 4
	if contentType == "application/octet-stream" && len(buf) >= 12 {
		if string(buf[4:8]) == "ftyp" {
			brand := string(buf[8:12])
			if brand == "heic" || brand == "heix" || brand == "hevc" || brand == "hevx" || brand == "mif1" {
				contentType = "image/heic"
			}
		}
	}

	// Validate supported content types
	if contentType != "image/jpeg" && contentType != "image/png" &&
		contentType != "image/heic" && contentType != "image/heif" {
		return "", "", fmt.Errorf("%w: unsupported file format", errInvalidPhoto)
	}

	// Decoders allocate from the dimensions in the header: refuse absurd ones first.
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	all, err := io.ReadAll(src)
	if err != nil {
		return "", "", err
	}
	if contentType == "image/heic" || contentType == "image/heif" {
		// The brand "mif1" has no registered decoder, so read the header explicitly.
		cfg, err := heic.DecodeConfig(bytes.NewReader(all))
		if err == nil {
			err = photofile.CheckSize(cfg.Width, cfg.Height)
		} else {
			err = fmt.Errorf("%w: %w", photofile.ErrUnreadable, err)
		}
		if err != nil {
			return "", "", fmt.Errorf("%w: %w", errInvalidPhoto, err)
		}
	} else if err := photofile.CheckDimensions(all); err != nil {
		return "", "", fmt.Errorf("%w: %w", errInvalidPhoto, err)
	}
	reader := bytes.NewReader(all)

	// Decode the image (handle JPEG, PNG, and HEIC)
	var img image.Image
	switch contentType {
	case "image/jpeg":
		img, err = jpeg.Decode(reader)
	case "image/png":
		img, err = png.Decode(reader)
	case "image/heic", "image/heif":
		img, err = heic.Decode(reader)
	}
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", errInvalidPhoto, err)
	}

	// Crop to centered square if rectangular
	img = cropToSquare(img)

	// Generate unique filename for full photo
	baseFilename := uuid.New().String()
	photoPath := baseFilename + "_photo.jpg" // Always save as JPG

	// Create the output directory
	if err := photofile.EnsureDir(uploadDir); err != nil {
		return "", "", err
	}

	// Save the photo as JPG (max 400x400, only downscale)
	const maxPhotoSize = 400
	bounds := img.Bounds()
	photoImg := img
	if bounds.Dx() > maxPhotoSize || bounds.Dy() > maxPhotoSize {
		photoImg = resizeTo(img, maxPhotoSize)
	}
	thumbnailPath := baseFilename + "_thumb.jpg"

	fullPhotoPath := filepath.Join(uploadDir, photoPath)
	if err := photofile.SaveJPEG(fullPhotoPath, photoImg); err != nil {
		return "", "", err
	}
	if err := photofile.SaveJPEG(filepath.Join(uploadDir, thumbnailPath), resizeTo(img, 48)); err != nil {
		os.Remove(fullPhotoPath)
		return "", "", err
	}

	return photoPath, thumbnailPath, nil
}

// cropToSquare crops an image to a centered square
// If the image is already square, it returns the original image unchanged
func cropToSquare(img image.Image) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Already square, return as-is
	if width == height {
		return img
	}

	// Calculate the size of the square (use the smaller dimension)
	size := width
	if height < width {
		size = height
	}

	// Calculate crop offset to center the square
	offsetX := (width - size) / 2
	offsetY := (height - size) / 2

	// Create a new RGBA image for the cropped result
	cropped := image.NewRGBA(image.Rect(0, 0, size, size))

	// Copy the centered square region
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			cropped.Set(x, y, img.At(bounds.Min.X+offsetX+x, bounds.Min.Y+offsetY+y))
		}
	}

	return cropped
}

// ProxyImage fetches an image from a URL and returns it to the client.
// This is used to work around CORS restrictions when fetching images from external URLs.
func ProxyImage(c *gin.Context) {
	imageURL := c.Query("url")
	if imageURL == "" {
		apperrors.AbortWithError(c, apperrors.ErrMissingField("url"))
		return
	}

	// Use the shared SSRF-protected fetch function
	body, contentType, err := httputil.FetchImageFromURL(imageURL)
	if err != nil {
		logger.FromContext(c).Warn().Err(err).Str("url", imageURL).Msg("Failed to fetch image from URL")
		apperrors.AbortWithError(c, apperrors.ErrExternal("image proxy", "Failed to fetch image").WithError(err))
		return
	}

	// Return the image with appropriate content type
	c.Data(http.StatusOK, contentType, body)
}

// resizeTo scales img to exactly size x size pixels. x/image/draw offers no
// Lanczos kernel; CatmullRom is the closest high-quality (bicubic) choice.
func resizeTo(img image.Image, size int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst
}
