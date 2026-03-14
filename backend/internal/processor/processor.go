package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/h2non/filetype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/yourusername/media-share/internal/storage"
)

type Job struct {
	MediaID  uuid.UUID
	S3Key    string
	MIMEType string
	UserID   uuid.UUID
}

type StatusUpdate struct {
	MediaID  string `json:"media_id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Error    string `json:"error,omitempty"`
}

type Processor struct {
	s3    *storage.S3Client
	db    *pgxpool.Pool
	redis *redis.Client
	jobs  chan Job
}

func New(s3 *storage.S3Client, db *pgxpool.Pool, rdb *redis.Client) *Processor {
	return &Processor{
		s3:    s3,
		db:    db,
		redis: rdb,
		jobs:  make(chan Job, 100),
	}
}

func (p *Processor) Start(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		go p.worker(ctx)
	}
}

func (p *Processor) Enqueue(job Job) {
	p.jobs <- job
}

func (p *Processor) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-p.jobs:
			p.process(ctx, job)
		}
	}
}

func (p *Processor) process(ctx context.Context, job Job) {
	p.setStatus(ctx, job.MediaID, "processing", 5, "")

	// Download from S3 to temp file
	tmpFile, err := os.CreateTemp("", "media-*")
	if err != nil {
		p.failMedia(ctx, job.MediaID, "failed to create temp file")
		return
	}
	defer os.Remove(tmpFile.Name())

	body, err := p.s3.GetObject(ctx, job.S3Key)
	if err != nil {
		p.failMedia(ctx, job.MediaID, "failed to download from S3")
		return
	}
	defer body.Close()

	if _, err = io.Copy(tmpFile, body); err != nil {
		p.failMedia(ctx, job.MediaID, "failed to write temp file")
		return
	}
	tmpFile.Close()

	p.setStatus(ctx, job.MediaID, "processing", 20, "")

	// Detect actual MIME type from magic bytes
	buf := make([]byte, 261)
	f, _ := os.Open(tmpFile.Name())
	f.Read(buf)
	f.Close()

	kind, _ := filetype.Match(buf)
	detectedMIME := kind.MIME.Value
	if detectedMIME == "" {
		detectedMIME = job.MIMEType // fallback
	}

	mediaType := classifyMIME(detectedMIME)

	switch mediaType {
	case "image":
		p.processImage(ctx, job, tmpFile.Name())
	case "video":
		p.processVideo(ctx, job, tmpFile.Name())
	default:
		p.failMedia(ctx, job.MediaID, "unsupported file type")
	}
}

func (p *Processor) processImage(ctx context.Context, job Job, tmpPath string) {
	src, err := imaging.Open(tmpPath, imaging.AutoOrientation(true))
	if err != nil {
		p.failMedia(ctx, job.MediaID, "failed to open image")
		return
	}

	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	p.setStatus(ctx, job.MediaID, "processing", 40, "")

	// Generate thumbnail (300px width)
	thumb := imaging.Resize(src, 300, 0, imaging.Lanczos)
	thumbKey := thumbnailKey(job.S3Key)
	if err := p.uploadImage(ctx, thumb, thumbKey); err != nil {
		p.failMedia(ctx, job.MediaID, "failed to upload thumbnail")
		return
	}

	p.setStatus(ctx, job.MediaID, "processing", 70, "")

	// Generate 1080p variant if larger
	if width > 1080 {
		resized := imaging.Resize(src, 1080, 0, imaging.Lanczos)
		resizedKey := variantKey(job.S3Key, "1080p")
		if err := p.uploadImage(ctx, resized, resizedKey); err != nil {
			p.failMedia(ctx, job.MediaID, "failed to upload 1080p variant")
			return
		}
		// Insert media_file record
		p.insertMediaFile(ctx, job.MediaID, "1080p", resizedKey, 1080, 0, "jpg")
	}

	p.insertMediaFile(ctx, job.MediaID, "thumb", thumbKey, 300, 0, "jpg")

	// Update media record
	_, err = p.db.Exec(ctx,
		`UPDATE media SET status='ready', width=$1, height=$2, updated_at=now()
		 WHERE id=$3`,
		width, height, job.MediaID,
	)
	if err != nil {
		p.failMedia(ctx, job.MediaID, "failed to update media record")
		return
	}

	// Update storage_used for user
	p.updateStorageUsed(ctx, job.MediaID, job.UserID)
	p.setStatus(ctx, job.MediaID, "ready", 100, "")
}

func (p *Processor) processVideo(ctx context.Context, job Job, tmpPath string) {
	p.setStatus(ctx, job.MediaID, "processing", 30, "")

	// Get video metadata via ffprobe
	duration, w, h, err := getVideoMetadata(tmpPath)
	if err != nil {
		p.failMedia(ctx, job.MediaID, "failed to read video metadata")
		return
	}

	p.setStatus(ctx, job.MediaID, "processing", 50, "")

	// Extract poster frame
	posterPath := tmpPath + "_poster.jpg"
	defer os.Remove(posterPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-i", tmpPath,
		"-vframes", "1",
		"-q:v", "2",
		posterPath,
	)
	if err := cmd.Run(); err != nil {
		// Non-fatal: continue without poster
	}

	if _, statErr := os.Stat(posterPath); statErr == nil {
		posterKey := thumbnailKey(job.S3Key)
		pf, _ := os.Open(posterPath)
		defer pf.Close()
		_ = p.s3.PutObject(ctx, posterKey, "image/jpeg", pf)
		p.insertMediaFile(ctx, job.MediaID, "poster", posterKey, w, h, "jpg")
	}

	p.setStatus(ctx, job.MediaID, "processing", 80, "")

	_, err = p.db.Exec(ctx,
		`UPDATE media SET status='ready', width=$1, height=$2, duration_sec=$3, updated_at=now()
		 WHERE id=$4`,
		w, h, int(duration.Seconds()), job.MediaID,
	)
	if err != nil {
		p.failMedia(ctx, job.MediaID, "failed to update media record")
		return
	}

	p.updateStorageUsed(ctx, job.MediaID, job.UserID)
	p.setStatus(ctx, job.MediaID, "ready", 100, "")
}

func (p *Processor) uploadImage(ctx context.Context, img image.Image, key string) error {
	buf := new(bytes.Buffer)
	if err := imaging.Encode(buf, img, imaging.JPEG, imaging.JPEGQuality(85)); err != nil {
		return err
	}
	return p.s3.PutObject(ctx, key, "image/jpeg", buf)
}

func (p *Processor) insertMediaFile(ctx context.Context, mediaID uuid.UUID, variant, key string, width, height int, format string) {
	_, _ = p.db.Exec(ctx,
		"INSERT INTO media_files (media_id, variant, s3_key, width, height, format) VALUES ($1,$2,$3,$4,$5,$6)",
		mediaID, variant, key, width, height, format,
	)
}

func (p *Processor) updateStorageUsed(ctx context.Context, mediaID, userID uuid.UUID) {
	_, _ = p.db.Exec(ctx,
		`UPDATE users SET storage_used = storage_used + (SELECT file_size FROM media WHERE id=$1)
		 WHERE id=$2`,
		mediaID, userID,
	)
}

func (p *Processor) setStatus(ctx context.Context, mediaID uuid.UUID, status string, progress int, errMsg string) {
	update := StatusUpdate{
		MediaID:  mediaID.String(),
		Status:   status,
		Progress: progress,
		Error:    errMsg,
	}
	data, _ := json.Marshal(update)
	key := fmt.Sprintf("process:%s", mediaID)
	p.redis.Set(ctx, key, string(data), 10*time.Minute)
}

func (p *Processor) failMedia(ctx context.Context, mediaID uuid.UUID, reason string) {
	_, _ = p.db.Exec(ctx, "UPDATE media SET status='failed', updated_at=now() WHERE id=$1", mediaID)
	p.setStatus(ctx, mediaID, "failed", 0, reason)
}

func (p *Processor) GetStatus(ctx context.Context, mediaID string) (*StatusUpdate, error) {
	key := fmt.Sprintf("process:%s", mediaID)
	val, err := p.redis.Get(ctx, key).Result()
	if err != nil {
		// Check DB as fallback
		var status string
		dbErr := p.db.QueryRow(ctx, "SELECT status FROM media WHERE id=$1", mediaID).Scan(&status)
		if dbErr != nil {
			return nil, fmt.Errorf("not found")
		}
		return &StatusUpdate{MediaID: mediaID, Status: status, Progress: 0}, nil
	}
	var update StatusUpdate
	if err := json.Unmarshal([]byte(val), &update); err != nil {
		return nil, err
	}
	return &update, nil
}

func thumbnailKey(originalKey string) string {
	ext := filepath.Ext(originalKey)
	base := strings.TrimSuffix(originalKey, ext)
	return base + "_thumb.jpg"
}

func variantKey(originalKey, variant string) string {
	ext := filepath.Ext(originalKey)
	base := strings.TrimSuffix(originalKey, ext)
	return fmt.Sprintf("%s_%s.jpg", base, variant)
}

func classifyMIME(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	default:
		return "unknown"
	}
}

func getVideoMetadata(path string) (duration time.Duration, width, height int, err error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, err
	}

	var probe struct {
		Streams []struct {
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return 0, 0, 0, err
	}

	for _, s := range probe.Streams {
		if s.Width > 0 {
			width = s.Width
			height = s.Height
			break
		}
	}

	var durationSec float64
	fmt.Sscanf(probe.Format.Duration, "%f", &durationSec)
	duration = time.Duration(durationSec * float64(time.Second))
	return
}
