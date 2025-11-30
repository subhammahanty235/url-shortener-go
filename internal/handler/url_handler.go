package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/subhammahanty235/url-shortener/internal/domain"
	"github.com/subhammahanty235/url-shortener/internal/service"
	"go.uber.org/zap"
)

type URLHandler struct {
	urlService *service.URLService
	logger     *zap.Logger
}

func NewURLHandler(
	urlService *service.URLService,
	logger *zap.Logger,
) *URLHandler {
	return &URLHandler{
		urlService: urlService,
		logger:     logger,
	}
}

func (h *URLHandler) CreateURL(c *gin.Context) {
	var req *domain.CreateURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Debug("invalid request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	resp, err := h.urlService.Create(c.Request.Context(), req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *URLHandler) RedirectURL(c *gin.Context) {
	shortCode := c.Param("shortCode")
	url, err := h.urlService.GetURL(c.Request.Context(), shortCode)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.Redirect(http.StatusMovedPermanently, url.OriginalURL)

}

func (h *URLHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrURLNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "URL not found",
		})
	case errors.Is(err, domain.ErrURLExpired):
		c.JSON(http.StatusGone, ErrorResponse{
			Error:   "expired",
			Message: "URL has expired",
		})
	case errors.Is(err, domain.ErrInvalidURL):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_url",
			Message: "Invalid URL format",
		})
	case errors.Is(err, domain.ErrShortCodeExists):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "conflict",
			Message: "Short code already exists",
		})
	case errors.Is(err, domain.ErrInvalidShortCode):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_short_code",
			Message: "Invalid short code format",
		})
	case errors.Is(err, domain.ErrRateLimitExceeded):
		c.JSON(http.StatusTooManyRequests, ErrorResponse{
			Error:   "rate_limit_exceeded",
			Message: "Rate limit exceeded",
		})
	default:
		h.logger.Error("unhandled error", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "An internal error occurred",
		})
	}
}

func (h *URLHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
