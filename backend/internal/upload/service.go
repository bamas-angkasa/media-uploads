package upload

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/yourusername/media-share/config"
	"github.com/yourusername/media-share/internal/processor"
	"github.com/yourusername/media-share/internal/shortcode"
	"github.com/yourusername/media-share/internal/storage"
)

var (
	ErrFileTooLarge      = errors.New("file exceeds size limit")
	ErrUnsupportedType   = errors.New("unsupported file type")
	ErrQuotaExceeded     = errors.New("storage quota exceeded")
	ErrUploadNotFound    = errors.New("upload session not found or expired")
	ErrS3ObjectMissing   = errors.New("file not found in storage")
)

var allowedMIMETypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/gif":  "gif",
	"video/mp4":  "mp4",
	"video/webm": "webm",
	"video/quicktime": "mov",
}

type SignRequest struct {
	Filename    string `json:"filename"     binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	SizeBytes   int64  `json:"size_bytes"   binding:"required,min=1"`
}

type SignResponse struct {
	UploadURL string `json:"upload_url"`
	MediaID   string `json:"media_id"`
	ShortCode string `json:"short_code"`
	ExpiresAt int64  `json:"expires_at"`
}

type Service struct {
	db        *pgxpool.Pool
	redis     *redis.Client
	s3        *storage.S3Client
	proc      *processor.Processor
	mediaCfg  config.MediaConfig
}

func NewService(db *pgxpool.Pool, rdb *redis.Client, s3 *storage.S3Client, proc *processor.Processor, cfg config.MediaConfig) *Service {
	return &Service{db: db, redis: rdb, s3: s3, proc: proc, mediaCfg: cfg}
}

func (s *Service) Sign(ctx context.Context, userID uuid.UUID, req SignRequest) (*SignResponse, error) {
	// Validate MIME type
	ext, ok := allowedMIMETypes[req.ContentType]
	if !ok {
		return nil, ErrUnsupportedType
	}

	// Validate size
	isVideo := strings.HasPrefix(req.ContentType, "video/")
	maxSize := s.mediaCfg.MaxImageSizeBytes
	if isVideo {
		maxSize = s.mediaCfg.MaxVideoSizeBytes
	}
	if req.SizeBytes > maxSize {
		return nil, ErrFileTooLarge
	}

	// Check quota
	var storageUsed, storageQuota int64
	if err := s.db.QueryRow(ctx, "SELECT storage_used, storage_quota FROM users WHERE id=$1", userID).
		Scan(&storageUsed, &storageQuota); err != nil {
		return nil, err
	}
	if storageUsed+req.SizeBytes > storageQuota {
		return nil, ErrQuotaExceeded
	}

	// Generate unique short code
	code, err := s.generateUniqueShortCode(ctx)
	if err != nil {
		return nil, err
	}

	// Determine media type
	mediaType := "image"
	if isVideo {
		mediaType = "video"
	} else if req.ContentType == "image/gif" {
		mediaType = "gif"
	}

	// Build S3 key
	now := time.Now()
	mediaID := uuid.New()
	s3Key := fmt.Sprintf("uploads/%s/%d/%02d/%s.%s",
		userID, now.Year(), now.Month(), mediaID, ext)

	// Insert media record
	_, err = s.db.Exec(ctx,
		`INSERT INTO media (id, user_id, short_code, type, status, original_key, file_size, mime_type)
		 VALUES ($1, $2, $3, $4, 'processing', $5, $6, $7)`,
		mediaID, userID, code, mediaType, s3Key, req.SizeBytes, req.ContentType,
	)
	if err != nil {
		return nil, err
	}

	// Store upload intent in Redis (20 min TTL)
	intentKey := fmt.Sprintf("upload:%s", mediaID)
	intentData := fmt.Sprintf("%s|%s|%s", s3Key, userID, req.ContentType)
	s.redis.Set(ctx, intentKey, intentData, 20*time.Minute)

	// Generate presigned URL
	expiresIn := 15 * time.Minute
	presignURL, err := s.s3.PresignPut(ctx, s3Key, req.ContentType, req.SizeBytes, expiresIn)
	if err != nil {
		return nil, err
	}

	return &SignResponse{
		UploadURL: presignURL,
		MediaID:   mediaID.String(),
		ShortCode: code,
		ExpiresAt: time.Now().Add(expiresIn).Unix(),
	}, nil
}

func (s *Service) Confirm(ctx context.Context, userID uuid.UUID, mediaIDStr string) error {
	mediaID, err := uuid.Parse(mediaIDStr)
	if err != nil {
		return errors.New("invalid media id")
	}

	intentKey := fmt.Sprintf("upload:%s", mediaID)
	intentData, err := s.redis.Get(ctx, intentKey).Result()
	if err != nil {
		return ErrUploadNotFound
	}

	parts := strings.SplitN(intentData, "|", 3)
	if len(parts) != 3 {
		return ErrUploadNotFound
	}
	s3Key, ownerIDStr, contentType := parts[0], parts[1], parts[2]

	// Verify ownership
	ownerID, _ := uuid.Parse(ownerIDStr)
	if ownerID != userID {
		return errors.New("unauthorized")
	}

	// Verify object exists in S3
	if err := s.s3.HeadObject(ctx, s3Key); err != nil {
		return ErrS3ObjectMissing
	}

	// Delete Redis intent
	s.redis.Del(ctx, intentKey)

	// Get file info from DB
	var fileSize int64
	if err := s.db.QueryRow(ctx, "SELECT file_size FROM media WHERE id=$1", mediaID).Scan(&fileSize); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("media not found")
		}
		return err
	}

	// Enqueue processing job
	s.proc.Enqueue(processor.Job{
		MediaID:  mediaID,
		S3Key:    s3Key,
		MIMEType: contentType,
		UserID:   userID,
	})

	return nil
}

func (s *Service) GetProcessingStatus(ctx context.Context, mediaIDStr string) (*processor.StatusUpdate, error) {
	return s.proc.GetStatus(ctx, mediaIDStr)
}

func (s *Service) generateUniqueShortCode(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		length := 8
		if attempt >= 3 {
			length = 10
		}
		code := shortcode.Generate(length)
		var count int
		if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM media WHERE short_code=$1", code).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return shortcode.Generate(12), nil
}

func mimeToExt(mime string) string {
	if ext, ok := allowedMIMETypes[mime]; ok {
		return ext
	}
	return filepath.Ext(mime)
}
