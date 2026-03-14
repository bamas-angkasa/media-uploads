package public

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/media-share/internal/auth"
	"github.com/yourusername/media-share/internal/media"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Explore(c *gin.Context) {
	cursor := c.Query("cursor")
	mediaType := c.Query("type")
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	items, nextCursor, err := h.svc.Explore(c.Request.Context(), ExploreParams{
		Cursor:   cursor,
		PageSize: pageSize,
		Type:     mediaType,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch explore feed"})
		return
	}
	if items == nil {
		items = []media.Media{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        items,
		"next_cursor": nextCursor,
	})
}

func (h *Handler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	items, total, err := h.svc.Search(c.Request.Context(), SearchParams{
		Query:    q,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}
	if items == nil {
		items = []media.Media{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *Handler) GetByShortCode(c *gin.Context) {
	shortCode := c.Param("short_code")
	m, err := h.svc.GetByShortCode(c.Request.Context(), shortCode)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "media not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch media"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": m})
}

func (h *Handler) RecordView(c *gin.Context) {
	shortCode := c.Param("short_code")
	go h.svc.RecordView(c.Request.Context(), shortCode)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) RecordDownload(c *gin.Context) {
	shortCode := c.Param("short_code")
	downloadURL, err := h.svc.RecordDownload(c.Request.Context(), shortCode)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "media not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get download URL"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"download_url": downloadURL})
}

func (h *Handler) CreateReport(c *gin.Context) {
	shortCode := c.Param("short_code")

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reporterID := auth.GetUserID(c)
	if err := h.svc.CreateReport(c.Request.Context(), shortCode, reporterID, req.Reason); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "media not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit report"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "report submitted"})
}
