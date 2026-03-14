package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type registerRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=8"`
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, user, err := h.svc.Register(c.Request.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "email already taken"})
		case errors.Is(err, ErrUsernameTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		}
		return
	}

	setRefreshCookie(c, pair.RefreshToken)
	c.JSON(http.StatusCreated, gin.H{
		"access_token": pair.AccessToken,
		"expires_in":   pair.ExpiresIn,
		"user":         userResponse(user),
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, user, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		case errors.Is(err, ErrAccountInactive):
			c.JSON(http.StatusForbidden, gin.H{"error": "account is suspended"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		}
		return
	}

	setRefreshCookie(c, pair.RefreshToken)
	c.JSON(http.StatusOK, gin.H{
		"access_token": pair.AccessToken,
		"expires_in":   pair.ExpiresIn,
		"user":         userResponse(user),
	})
}

func (h *Handler) Logout(c *gin.Context) {
	token, err := c.Cookie("refresh_token")
	if err == nil && token != "" {
		_ = h.svc.Logout(c.Request.Context(), token)
	}
	clearRefreshCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *Handler) Refresh(c *gin.Context) {
	token, err := c.Cookie("refresh_token")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
		return
	}

	pair, err := h.svc.Refresh(c.Request.Context(), token)
	if err != nil {
		clearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}

	setRefreshCookie(c, pair.RefreshToken)
	c.JSON(http.StatusOK, gin.H{
		"access_token": pair.AccessToken,
		"expires_in":   pair.ExpiresIn,
	})
}

func (h *Handler) Me(c *gin.Context) {
	userID := GetUserID(c)
	user, err := h.svc.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userResponse(user)})
}

func userResponse(u *User) gin.H {
	return gin.H{
		"id":            u.ID,
		"email":         u.Email,
		"username":      u.Username,
		"role":          u.Role,
		"plan":          u.Plan,
		"storage_quota": u.StorageQuota,
		"storage_used":  u.StorageUsed,
		"created_at":    u.CreatedAt,
	}
}

func setRefreshCookie(c *gin.Context, token string) {
	c.SetCookie("refresh_token", token, 7*24*3600, "/", "", true, true)
}

func clearRefreshCookie(c *gin.Context) {
	c.SetCookie("refresh_token", "", -1, "/", "", true, true)
}
