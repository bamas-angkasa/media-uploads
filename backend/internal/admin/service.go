package admin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/media-share/internal/storage"
)

type MediaFilter struct {
	UserID   string
	Status   string
	Type     string
	Search   string
	Page     int
	PageSize int
	SortBy   string
	Order    string
}

type UserUpdate struct {
	IsActive     *bool   `json:"is_active"`
	Role         *string `json:"role"`
	StorageQuota *int64  `json:"storage_quota"`
}

type ReportAction struct {
	Action string `json:"action" binding:"required"` // "resolve" | "dismiss"
}

type PlatformStats struct {
	TotalMedia      int64 `json:"total_media"`
	TotalUsers      int64 `json:"total_users"`
	TotalStorageGB  float64 `json:"total_storage_gb"`
	MediaLast24h    int64 `json:"media_last_24h"`
	UsersLast7d     int64 `json:"users_last_7d"`
	PendingReports  int64 `json:"pending_reports"`
}

type Service struct {
	db *pgxpool.Pool
	s3 *storage.S3Client
}

func NewService(db *pgxpool.Pool, s3 *storage.S3Client) *Service {
	return &Service{db: db, s3: s3}
}

func (s *Service) ListMedia(ctx context.Context, f MediaFilter) ([]map[string]interface{}, int, error) {
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}
	offset := (f.Page - 1) * f.PageSize
	if offset < 0 {
		offset = 0
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	idx := 1

	if f.UserID != "" {
		where += fmt.Sprintf(" AND m.user_id=$%d", idx)
		args = append(args, f.UserID)
		idx++
	}
	if f.Status != "" {
		where += fmt.Sprintf(" AND m.status=$%d", idx)
		args = append(args, f.Status)
		idx++
	}
	if f.Type != "" {
		where += fmt.Sprintf(" AND m.type=$%d", idx)
		args = append(args, f.Type)
		idx++
	}
	if f.Search != "" {
		where += fmt.Sprintf(" AND (m.title ILIKE $%d OR m.short_code=$%d)", idx, idx+1)
		args = append(args, "%"+f.Search+"%", f.Search)
		idx += 2
	}

	sortBy := "m.created_at"
	if f.SortBy == "view_count" {
		sortBy = "m.view_count"
	}
	order := "DESC"
	if f.Order == "asc" {
		order = "ASC"
	}

	countQuery := `SELECT COUNT(*) FROM media m ` + where
	var total int
	s.db.QueryRow(ctx, countQuery, args...).Scan(&total)

	dataArgs := append(args, f.PageSize, offset)
	rows, err := s.db.Query(ctx,
		fmt.Sprintf(`
		SELECT m.id, m.short_code, m.type, m.title, m.status, m.file_size,
		       m.view_count, m.download_count, m.created_at,
		       u.username, u.email
		FROM media m
		JOIN users u ON m.user_id = u.id
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
			where, sortBy, order, idx, idx+1),
		dataArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var id, shortCode, typ, status string
		var title *string
		var fileSize, viewCount, downloadCount int64
		var createdAt time.Time
		var username, email string

		rows.Scan(&id, &shortCode, &typ, &title, &status, &fileSize, &viewCount, &downloadCount, &createdAt, &username, &email)
		result = append(result, map[string]interface{}{
			"id": id, "short_code": shortCode, "type": typ, "title": title,
			"status": status, "file_size": fileSize, "view_count": viewCount,
			"download_count": downloadCount, "created_at": createdAt,
			"username": username, "email": email,
		})
	}
	return result, total, rows.Err()
}

func (s *Service) DeleteMedia(ctx context.Context, id uuid.UUID) error {
	var originalKey string
	if err := s.db.QueryRow(ctx, "SELECT original_key FROM media WHERE id=$1", id).Scan(&originalKey); err != nil {
		return errors.New("media not found")
	}

	_ = s.s3.DeleteObject(ctx, originalKey)

	rows, _ := s.db.Query(ctx, "SELECT s3_key FROM media_files WHERE media_id=$1", id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			rows.Scan(&key)
			_ = s.s3.DeleteObject(ctx, key)
		}
	}

	_, err := s.db.Exec(ctx, "DELETE FROM media WHERE id=$1", id)
	return err
}

func (s *Service) ListUsers(ctx context.Context, page, pageSize int, search string) ([]map[string]interface{}, int, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	idx := 1

	if search != "" {
		where += fmt.Sprintf(" AND (username ILIKE $%d OR email ILIKE $%d)", idx, idx+1)
		args = append(args, "%"+search+"%", "%"+search+"%")
		idx += 2
	}

	var total int
	s.db.QueryRow(ctx, "SELECT COUNT(*) FROM users "+where, args...).Scan(&total)

	dataArgs := append(args, pageSize, offset)
	rows, err := s.db.Query(ctx,
		fmt.Sprintf(`
		SELECT id, email, username, role, plan, storage_quota, storage_used, is_active, created_at
		FROM users %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
			where, idx, idx+1),
		dataArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var id, email, username, role, plan string
		var storageQuota, storageUsed int64
		var isActive bool
		var createdAt time.Time

		rows.Scan(&id, &email, &username, &role, &plan, &storageQuota, &storageUsed, &isActive, &createdAt)
		result = append(result, map[string]interface{}{
			"id": id, "email": email, "username": username, "role": role,
			"plan": plan, "storage_quota": storageQuota, "storage_used": storageUsed,
			"is_active": isActive, "created_at": createdAt,
		})
	}
	return result, total, rows.Err()
}

func (s *Service) UpdateUser(ctx context.Context, id uuid.UUID, req UserUpdate) error {
	if req.IsActive != nil {
		_, err := s.db.Exec(ctx, "UPDATE users SET is_active=$1, updated_at=now() WHERE id=$2", *req.IsActive, id)
		if err != nil {
			return err
		}
	}
	if req.Role != nil {
		_, err := s.db.Exec(ctx, "UPDATE users SET role=$1, updated_at=now() WHERE id=$2", *req.Role, id)
		if err != nil {
			return err
		}
	}
	if req.StorageQuota != nil {
		_, err := s.db.Exec(ctx, "UPDATE users SET storage_quota=$1, updated_at=now() WHERE id=$2", *req.StorageQuota, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListReports(ctx context.Context, status string, page, pageSize int) ([]map[string]interface{}, int, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	idx := 1

	if status != "" {
		where += fmt.Sprintf(" AND r.status=$%d", idx)
		args = append(args, status)
		idx++
	}

	var total int
	s.db.QueryRow(ctx, "SELECT COUNT(*) FROM reports r "+where, args...).Scan(&total)

	dataArgs := append(args, pageSize, offset)
	rows, err := s.db.Query(ctx,
		fmt.Sprintf(`
		SELECT r.id, r.reason, r.status, r.created_at,
		       m.id AS media_id, m.short_code, m.title,
		       u.username AS reporter_username,
		       mf.s3_key AS thumb_key
		FROM reports r
		JOIN media m ON r.media_id = m.id
		JOIN users u ON r.reporter_id = u.id
		LEFT JOIN media_files mf ON mf.media_id = m.id AND mf.variant = 'thumbnail'
		%s ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d`,
			where, idx, idx+1),
		dataArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var rID, reason, rStatus, mediaID, shortCode, reporterUsername string
		var title, thumbKey *string
		var createdAt time.Time

		rows.Scan(&rID, &reason, &rStatus, &createdAt, &mediaID, &shortCode, &title, &reporterUsername, &thumbKey)

		thumbnailURL := ""
		if thumbKey != nil && *thumbKey != "" {
			thumbnailURL = s.s3.CDNUrl(*thumbKey)
		}

		result = append(result, map[string]interface{}{
			"id": rID, "reason": reason, "status": rStatus, "created_at": createdAt,
			"media_id": mediaID, "media_short_code": shortCode, "media_title": title,
			"reporter": reporterUsername, "thumbnail_url": thumbnailURL,
		})
	}
	return result, total, rows.Err()
}

func (s *Service) UpdateReport(ctx context.Context, id uuid.UUID, reviewerID uuid.UUID, action string) error {
	status := "dismissed"
	if action == "resolve" {
		status = "resolved"
	}
	_, err := s.db.Exec(ctx,
		"UPDATE reports SET status=$1, reviewed_by=$2, reviewed_at=now() WHERE id=$3",
		status, reviewerID, id,
	)
	return err
}

func (s *Service) GetStats(ctx context.Context) (*PlatformStats, error) {
	stats := &PlatformStats{}
	var storageBytes int64

	err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status='ready') AS total_media,
			COUNT(*) FILTER (WHERE created_at > now() - interval '24 hours') AS media_last_24h,
			COALESCE(SUM(file_size) FILTER (WHERE status='ready'), 0) AS total_storage
		FROM media
	`).Scan(&stats.TotalMedia, &stats.MediaLast24h, &storageBytes)
	if err != nil {
		return nil, err
	}
	stats.TotalStorageGB = float64(storageBytes) / (1024 * 1024 * 1024)

	s.db.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total_users,
			COUNT(*) FILTER (WHERE created_at > now() - interval '7 days') AS users_last_7d
		FROM users
	`).Scan(&stats.TotalUsers, &stats.UsersLast7d)

	s.db.QueryRow(ctx, "SELECT COUNT(*) FROM reports WHERE status='pending'").Scan(&stats.PendingReports)

	return stats, nil
}
