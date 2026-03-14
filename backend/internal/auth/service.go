package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/yourusername/media-share/config"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already taken")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrAccountInactive    = errors.New("account is suspended")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

type User struct {
	ID           uuid.UUID
	Email        string
	Username     string
	Role         string
	Plan         string
	StorageQuota int64
	StorageUsed  int64
	IsActive     bool
	CreatedAt    time.Time
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type Service struct {
	db    *pgxpool.Pool
	redis *redis.Client
	cfg   config.JWTConfig
}

func NewService(db *pgxpool.Pool, rdb *redis.Client, cfg config.JWTConfig) *Service {
	return &Service{db: db, redis: rdb, cfg: cfg}
}

func (s *Service) Register(ctx context.Context, email, username, password string) (*TokenPair, *User, error) {
	// Check uniqueness
	var count int
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email=$1", email).Scan(&count); err != nil {
		return nil, nil, err
	}
	if count > 0 {
		return nil, nil, ErrEmailTaken
	}
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE username=$1", username).Scan(&count); err != nil {
		return nil, nil, err
	}
	if count > 0 {
		return nil, nil, ErrUsernameTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, nil, err
	}

	user := &User{}
	err = s.db.QueryRow(ctx,
		`INSERT INTO users (email, username, password_hash) VALUES ($1, $2, $3)
		 RETURNING id, email, username, role, plan, storage_quota, storage_used, is_active, created_at`,
		email, username, string(hash),
	).Scan(&user.ID, &user.Email, &user.Username, &user.Role, &user.Plan, &user.StorageQuota, &user.StorageUsed, &user.IsActive, &user.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	pair, err := s.issueTokenPair(ctx, user)
	return pair, user, err
}

func (s *Service) Login(ctx context.Context, email, password string) (*TokenPair, *User, error) {
	user := &User{}
	var passwordHash string
	err := s.db.QueryRow(ctx,
		`SELECT id, email, username, role, plan, storage_quota, storage_used, is_active, password_hash
		 FROM users WHERE email=$1`,
		email,
	).Scan(&user.ID, &user.Email, &user.Username, &user.Role, &user.Plan, &user.StorageQuota, &user.StorageUsed, &user.IsActive, &passwordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, err
	}

	if !user.IsActive {
		return nil, nil, ErrAccountInactive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	pair, err := s.issueTokenPair(ctx, user)
	return pair, user, err
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := hashToken(refreshToken)
	_, err := s.db.Exec(ctx,
		"UPDATE refresh_tokens SET revoked=true WHERE token_hash=$1",
		tokenHash,
	)
	return err
}

func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (*TokenPair, error) {
	claims, err := s.parseRefreshToken(rawRefreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	userIDStr, _ := claims.GetSubject()
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, ErrInvalidToken
	}

	tokenHash := hashToken(rawRefreshToken)

	// Validate in DB
	var revoked bool
	var expiresAt time.Time
	err = s.db.QueryRow(ctx,
		"SELECT revoked, expires_at FROM refresh_tokens WHERE token_hash=$1 AND user_id=$2",
		tokenHash, userID,
	).Scan(&revoked, &expiresAt)
	if err != nil || revoked || time.Now().After(expiresAt) {
		return nil, ErrInvalidToken
	}

	// Revoke old token
	_, err = s.db.Exec(ctx, "UPDATE refresh_tokens SET revoked=true WHERE token_hash=$1", tokenHash)
	if err != nil {
		return nil, err
	}

	// Load user
	user := &User{}
	err = s.db.QueryRow(ctx,
		"SELECT id, email, username, role, plan, storage_quota, storage_used, is_active FROM users WHERE id=$1",
		userID,
	).Scan(&user.ID, &user.Email, &user.Username, &user.Role, &user.Plan, &user.StorageQuota, &user.StorageUsed, &user.IsActive)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrAccountInactive
	}

	return s.issueTokenPair(ctx, user)
}

func (s *Service) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user := &User{}
	err := s.db.QueryRow(ctx,
		"SELECT id, email, username, role, plan, storage_quota, storage_used, is_active, created_at FROM users WHERE id=$1",
		id,
	).Scan(&user.ID, &user.Email, &user.Username, &user.Role, &user.Plan, &user.StorageQuota, &user.StorageUsed, &user.IsActive, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return user, err
}

func (s *Service) issueTokenPair(ctx context.Context, user *User) (*TokenPair, error) {
	// Access token
	accessClaims := jwt.MapClaims{
		"sub":  user.ID.String(),
		"role": user.Role,
		"exp":  time.Now().Add(s.cfg.AccessTTL).Unix(),
		"iat":  time.Now().Unix(),
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return nil, err
	}

	// Refresh token
	refreshID := uuid.NewString()
	refreshClaims := jwt.MapClaims{
		"sub": user.ID.String(),
		"jti": refreshID,
		"exp": time.Now().Add(s.cfg.RefreshTTL).Unix(),
	}
	rawRefresh, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return nil, err
	}

	// Store hashed refresh token
	expiresAt := time.Now().Add(s.cfg.RefreshTTL)
	_, err = s.db.Exec(ctx,
		"INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)",
		user.ID, hashToken(rawRefresh), expiresAt,
	)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int(s.cfg.AccessTTL.Seconds()),
	}, nil
}

func (s *Service) parseRefreshToken(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum)
}
