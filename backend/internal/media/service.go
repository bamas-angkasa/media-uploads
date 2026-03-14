package media

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/media-share/internal/storage"
)

var ErrNotFound = errors.New("media not found")
var ErrForbidden = errors.New("access denied")

type Media struct {
	ID            uuid.UUID  `json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	ShortCode     string     `json:"short_code"`
	Type          string     `json:"type"`
	Title         *string    `json:"title"`
	Description   *string    `json:"description"`
	Tags          []string   `json:"tags"`
	Status        string     `json:"status"`
	OriginalKey   string     `json:"-"`
	FileSize      int64      `json:"file_size"`
	MimeType      string     `json:"mime_type"`
	Width         *int       `json:"width"`
	Height        *int       `json:"height"`
	DurationSec   *int       `json:"duration_sec"`
	ViewCount     int64      `json:"view_count"`
	DownloadCount int64      `json:"download_count"`
	ThumbnailURL  string     `json:"thumbnail_url,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type UpdateRequest struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Tags        []string `json:"tags"`
}

type ListParams struct {
	UserID   uuid.UUID
	Page     int
	PageSize int
}

type Service struct {
	db *pgxpool.Pool
	s3 *storage.S3Client
}

func NewService(db *pgxpool.Pool, s3 *storage.S3Client) *Service {
	return &Service{db: db, s3: s3}
}

func (s *Service) List(ctx context.Context, params ListParams) ([]Media, error) {
	if params.PageSize <= 0 || params.PageSize > 50 {
		params.PageSize = 20
	}
	offset := (params.Page - 1) * params.PageSize
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, short_code, type, title, description, tags, status,
		        original_key, file_size, mime_type, width, height, duration_sec,
		        view_count, download_count, created_at, updated_at
		 FROM media WHERE user_id=$1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		params.UserID, params.PageSize, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRows(ctx, rows)
}

func (s *Service) Get(ctx context.Context, id, userID uuid.UUID) (*Media, error) {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.UserID != userID {
		return nil, ErrForbidden
	}
	s.enrichWithURLs(ctx, m)
	return m, nil
}

func (s *Service) Update(ctx context.Context, id, userID uuid.UUID, req UpdateRequest) (*Media, error) {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.UserID != userID {
		return nil, ErrForbidden
	}

	_, err = s.db.Exec(ctx,
		`UPDATE media SET title=$1, description=$2, tags=$3, updated_at=now()
		 WHERE id=$4`,
		req.Title, req.Description, req.Tags, id,
	)
	if err != nil {
		return nil, err
	}

	return s.Get(ctx, id, userID)
}

func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return err
	}
	if m.UserID != userID {
		return ErrForbidden
	}

	// Delete S3 objects
	_ = s.s3.DeleteObject(ctx, m.OriginalKey)

	// Get and delete variants
	rows, err := s.db.Query(ctx, "SELECT s3_key FROM media_files WHERE media_id=$1", id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			rows.Scan(&key)
			_ = s.s3.DeleteObject(ctx, key)
		}
	}

	// Update storage used
	_, _ = s.db.Exec(ctx,
		"UPDATE users SET storage_used = GREATEST(0, storage_used - $1) WHERE id=$2",
		m.FileSize, userID,
	)

	_, err = s.db.Exec(ctx, "DELETE FROM media WHERE id=$1", id)
	return err
}

func (s *Service) getByID(ctx context.Context, id uuid.UUID) (*Media, error) {
	m := &Media{}
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, short_code, type, title, description, tags, status,
		        original_key, file_size, mime_type, width, height, duration_sec,
		        view_count, download_count, created_at, updated_at
		 FROM media WHERE id=$1`,
		id,
	).Scan(&m.ID, &m.UserID, &m.ShortCode, &m.Type, &m.Title, &m.Description, &m.Tags, &m.Status,
		&m.OriginalKey, &m.FileSize, &m.MimeType, &m.Width, &m.Height, &m.DurationSec,
		&m.ViewCount, &m.DownloadCount, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (s *Service) enrichWithURLs(ctx context.Context, m *Media) {
	// Get thumbnail URL from media_files
	var thumbKey string
	if err := s.db.QueryRow(ctx,
		"SELECT s3_key FROM media_files WHERE media_id=$1 AND variant IN ('thumb','poster') LIMIT 1",
		m.ID,
	).Scan(&thumbKey); err == nil {
		m.ThumbnailURL = s.s3.CDNUrl(thumbKey)
	}
}

func (s *Service) scanRows(ctx context.Context, rows pgx.Rows) ([]Media, error) {
	var result []Media
	for rows.Next() {
		m := Media{}
		err := rows.Scan(&m.ID, &m.UserID, &m.ShortCode, &m.Type, &m.Title, &m.Description, &m.Tags, &m.Status,
			&m.OriginalKey, &m.FileSize, &m.MimeType, &m.Width, &m.Height, &m.DurationSec,
			&m.ViewCount, &m.DownloadCount, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, err
		}
		s.enrichWithURLs(ctx, &m)
		result = append(result, m)
	}
	return result, rows.Err()
}
