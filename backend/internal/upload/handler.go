package upload

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/media-share/internal/auth"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Sign(c *gin.Context) {
	var req SignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := auth.GetUserID(c)
	resp, err := h.svc.Sign(c.Request.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnsupportedType):
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file type"})
		case errors.Is(err, ErrFileTooLarge):
			c.JSON(http.StatusBadRequest, gin.H{"error": "file exceeds size limit"})
		case errors.Is(err, ErrQuotaExceeded):
			c.JSON(http.StatusForbidden, gin.H{"error": "storage quota exceeded"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate upload URL"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Confirm(c *gin.Context) {
	var req struct {
		MediaID string `json:"media_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := auth.GetUserID(c)
	if err := h.svc.Confirm(c.Request.Context(), userID, req.MediaID); err != nil {
		switch {
		case errors.Is(err, ErrUploadNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "upload session not found or expired"})
		case errors.Is(err, ErrS3ObjectMissing):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "file not found in storage, please upload again"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to confirm upload"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "processing started", "media_id": req.MediaID})
}

// Progress streams SSE events for media processing status.
func (h *Handler) Progress(c *gin.Context) {
	mediaID := c.Param("id")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status, err := h.svc.GetProcessingStatus(ctx, mediaID)
			if err != nil {
				fmt.Fprintf(c.Writer, "data: {\"error\": \"not found\"}\n\n")
				c.Writer.Flush()
				return
			}

			data, _ := json.Marshal(status)
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()

			if status.Status == "ready" || status.Status == "failed" {
				return
			}
		}
	}
}
