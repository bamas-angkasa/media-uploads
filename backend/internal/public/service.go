package public

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/yourusername/media-share/internal/media"
	"github.com/yourusername/media-share/internal/storage"
)

var ErrNotFound = errors.New("media not found")

type ExploreParams struct {
	Cursor   string
	PageSize int
	Type     string // "" | "image" | "video" | "gif"
}

type SearchParams struct {
	Query    string
	Page     int
	PageSize int
}

type Service struct {
	db    *pgxpool.Pool
	redis *redis.Client
	s3    *storage.S3Client
}

func NewService(db *pgxpool.Pool, rdb *redis.Client, s3 *storage.S3Client) *Service {
	return &Service{db: db, redis: rdb, s3: s3}
}

func (s *Service) Explore(ctx context.Context, params ExploreParams) ([]media.Media, string, error) {
	if params.PageSize <= 0 || params.PageSize > 50 {
		params.PageSize = 20
	}

	query := `SELECT id, user_id, short_code, type, title, description, tags, status,
	                 original_key, file_size, mime_type, width, height, duration_sec,
	                 view_count, download_count, created_at, updated_at
	          FROM media
	          WHERE status='ready'`

	args := []interface{}{}
	argIdx := 1

	if params.Type != "" {
		query += fmt.Sprintf(" AND type=$%d", argIdx)
		args = append(args, params.Type)
		argIdx++
	}

	if params.Cursor != "" {
		// cursor = "createdAt_id"
		var cursorTime time.Time
		var cursorID uuid.UUID
		_, err := fmt.Sscanf(params.Cursor, "%s_%s", &cursorTime, &cursorID)
		if err == nil {
			query += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1)
			args = append(args, cursorTime, cursorID)
			argIdx += 2
		}
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", argIdx)
	args = append(args, params.PageSize+1) // fetch one extra to know if there's a next page

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items, err := s.scanMediaRows(ctx, rows)
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(items) > params.PageSize {
		items = items[:params.PageSize]
		last := items[len(items)-1]
		nextCursor = fmt.Sprintf("%s_%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}

	return items, nextCursor, nil
}

func (s *Service) Search(ctx context.Context, params SearchParams) ([]media.Media, int, error) {
	if params.PageSize <= 0 || params.PageSize > 50 {
		params.PageSize = 20
	}
	offset := (params.Page - 1) * params.PageSize
	if offset < 0 {
		offset = 0
	}

	// Count
	var total int
	s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM media
		 WHERE status='ready'
		 AND to_tsvector('english', coalesce(title,'') || ' ' || coalesce(description,''))
		     @@ plainto_tsquery('english', $1)`,
		params.Query,
	).Scan(&total)

	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, short_code, type, title, description, tags, status,
		        original_key, file_size, mime_type, width, height, duration_sec,
		        view_count, download_count, created_at, updated_at
		 FROM media
		 WHERE status='ready'
		 AND to_tsvector('english', coalesce(title,'') || ' ' || coalesce(description,''))
		     @@ plainto_tsquery('english', $1)
		 ORDER BY ts_rank(
		     to_tsvector('english', coalesce(title,'') || ' ' || coalesce(description,'')),
		     plainto_tsquery('english', $1)
		 ) DESC
		 LIMIT $2 OFFSET $3`,
		params.Query, params.PageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := s.scanMediaRows(ctx, rows)
	return items, total, err
}

func (s *Service) GetByShortCode(ctx context.Context, shortCode string) (*media.Media, error) {
	m := &media.Media{}
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, short_code, type, title, description, tags, status,
		        original_key, file_size, mime_type, width, height, duration_sec,
		        view_count, download_count, created_at, updated_at
		 FROM media WHERE short_code=$1 AND status='ready'`,
		shortCode,
	).Scan(&m.ID, &m.UserID, &m.ShortCode, &m.Type, &m.Title, &m.Description, &m.Tags, &m.Status,
		&m.OriginalKey, &m.FileSize, &m.MimeType, &m.Width, &m.Height, &m.DurationSec,
		&m.ViewCount, &m.DownloadCount, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.enrichWithURLs(ctx, m)
	return m, nil
}

func (s *Service) RecordView(ctx context.Context, shortCode string) {
	var mediaID uuid.UUID
	if err := s.db.QueryRow(ctx, "SELECT id FROM media WHERE short_code=$1", shortCode).Scan(&mediaID); err != nil {
		return
	}
	key := fmt.Sprintf("media:views:%s", mediaID)
	s.redis.Incr(ctx, key)
	s.redis.Expire(ctx, key, 24*time.Hour)
}

func (s *Service) RecordDownload(ctx context.Context, shortCode string) (string, error) {
	var mediaID uuid.UUID
	var originalKey string
	err := s.db.QueryRow(ctx, "SELECT id, original_key FROM media WHERE short_code=$1 AND status='ready'", shortCode).
		Scan(&mediaID, &originalKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	_, err = s.db.Exec(ctx, "UPDATE media SET download_count=download_count+1 WHERE id=$1", mediaID)
	if err != nil {
		return "", err
	}

	downloadURL, err := s.s3.PresignGet(ctx, originalKey, time.Hour)
	return downloadURL, err
}

// StartViewFlusher periodically flushes buffered view counts from Redis to Postgres.
func (s *Service) StartViewFlusher(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flushViews(ctx)
		}
	}
}

func (s *Service) CreateReport(ctx context.Context, shortCode string, reporterID uuid.UUID, reason string) error {
	m, err := s.GetByShortCode(ctx, shortCode)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx,
		"INSERT INTO reports (media_id, reporter_id, reason) VALUES ($1, $2, $3)",
		m.ID, reporterID, reason,
	)
	return err
}

func (s *Service) flushViews(ctx context.Context) {
	keys, err := s.redis.Keys(ctx, "media:views:*").Result()
	if err != nil || len(keys) == 0 {
		return
	}

	for _, key := range keys {
		val, err := s.redis.GetDel(ctx, key).Result()
		if err != nil {
			continue
		}
		var count int64
		fmt.Sscanf(val, "%d", &count)
		if count <= 0 {
			continue
		}
		mediaIDStr := key[len("media:views:"):]
		s.db.Exec(ctx,
			"UPDATE media SET view_count=view_count+$1 WHERE id=$2",
			count, mediaIDStr,
		)
	}
}

func (s *Service) enrichWithURLs(ctx context.Context, m *media.Media) {
	var thumbKey string
	if err := s.db.QueryRow(ctx,
		"SELECT s3_key FROM media_files WHERE media_id=$1 AND variant IN ('thumb','poster') LIMIT 1",
		m.ID,
	).Scan(&thumbKey); err == nil {
		m.ThumbnailURL = s.s3.CDNUrl(thumbKey)
	}
}

func (s *Service) scanMediaRows(_ context.Context, rows pgx.Rows) ([]media.Media, error) {
	var result []media.Media
	for rows.Next() {
		m := media.Media{}
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
