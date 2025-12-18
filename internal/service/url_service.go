package service

import (
	"context"
	"strings"
	"time"

	"github.com/subhammahanty235/url-shortener/internal/domain"
	"github.com/subhammahanty235/url-shortener/internal/pkg/keygen"
	"github.com/subhammahanty235/url-shortener/internal/pkg/metrics"
	"go.uber.org/zap"
)

type URLService struct {
	urlRepo     domain.URLRepository
	cacheRepo   domain.CacheRepository
	keyGen      *keygen.SnowFlakeGenerator
	logger      *zap.Logger
	metrics     *metrics.Metrics
	baseURL     string
	defaultTTL  time.Duration
	maxTTL      time.Duration
	cacheTTL    time.Duration
	allowCustom bool
}

type URLServiceConfig struct {
	BaseURL     string
	DefaultTTL  time.Duration
	MaxTTL      time.Duration
	AllowCustom bool
	CacheTTL    time.Duration
}

func NewURLService(
	urlRepo domain.URLRepository,
	cacheRepo domain.CacheRepository,
	keyGen *keygen.SnowFlakeGenerator,
	logger *zap.Logger,
	m *metrics.Metrics,
	cfg URLServiceConfig,
) *URLService {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 24 * time.Hour
	}

	return &URLService{
		urlRepo:     urlRepo,
		cacheRepo:   cacheRepo,
		keyGen:      keyGen,
		logger:      logger,
		metrics:     m,
		baseURL:     strings.TrimSuffix(cfg.BaseURL, "/"),
		defaultTTL:  cfg.DefaultTTL,
		maxTTL:      cfg.MaxTTL,
		allowCustom: cfg.AllowCustom,
		cacheTTL:    cfg.CacheTTL,
	}
}

func (s *URLService) Create(ctx context.Context, req *domain.CreateURLRequest) (*domain.CreateURLResponse, error) {

	var shortCode string
	var err error
	isCustomAlias := false

	if req.CustomAlias != nil && *req.CustomAlias != "" {
		shortCode = *req.CustomAlias
		isCustomAlias = true
		// TODO: check if the custom short code already exists
	} else {
		shortCode, err = s.keyGen.Generate()
		if err != nil {
			s.logger.Error("failed to generate short code", zap.Error(err))
			return nil, err

		}
	}

	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		ttl := time.Duration(*req.ExpiresIn) * time.Second
		if s.maxTTL > 0 && ttl > s.maxTTL {
			ttl = s.maxTTL
		}
		exp := time.Now().Add(ttl)
		expiresAt = &exp
	} else if s.defaultTTL > 0 {
		exp := time.Now().Add(s.defaultTTL)
		expiresAt = &exp
	}

	urlEntry := &domain.URL{
		ShortURL:    shortCode,
		OriginalURL: req.OriginalURL,
		ExpiresAt:   expiresAt,
		IsActive:    true,
	}

	if err := s.urlRepo.Create(ctx, urlEntry); err != nil {
		s.logger.Error("failed to create url entry", zap.Error(err))
		return nil, err
	}

	if err := s.cacheRepo.Set(ctx, urlEntry, s.cacheTTL); err != nil {
		s.logger.Error("failed to set url entry in cache", zap.Error(err))
		return nil, err
	}

	// Track business metrics
	// Learning: These metrics answer "how is our product being used?"
	s.metrics.URLsCreatedTotal.Inc()
	if isCustomAlias {
		// Track custom alias usage separately
		// Use case: Understand feature adoption - are users using custom aliases?
		s.metrics.CustomAliasTotal.Inc()
	}

	s.logger.Info("URL created successfully", zap.String("short_code", shortCode), zap.String("original_url", req.OriginalURL))

	return &domain.CreateURLResponse{
		ShortCode:   shortCode,
		ShortURL:    s.baseURL + "/" + shortCode,
		OriginalURL: req.OriginalURL,
		ExpiresAt:   expiresAt,
		CreatedAt:   urlEntry.CreatedAt,
	}, nil
}

func (s *URLService) GetURL(ctx context.Context, shortCode string) (*domain.URL, error) {
	// query the cache first
	url, err := s.cacheRepo.Get(ctx, shortCode)
	if err != nil {
		s.logger.Warn("cache error", zap.Error(err), zap.String("short_code", shortCode))
	}

	if url != nil {
		// Cache hit!
		s.logger.Debug("cache hit", zap.String("short_code", shortCode))

		if url.IsExpired() {
			_ = s.cacheRepo.Delete(ctx, shortCode)
			// Track expired URL attempts (important user experience metric)
			s.metrics.ExpiredURLsTotal.Inc()
			return nil, domain.ErrURLExpired
		}

		// Track redirect for cache hit
		// Learning: Most redirects should be cache hits for good performance
		s.metrics.URLRedirectsTotal.Inc()
		return url, nil
	}

	// Cache miss - need to query database
	s.logger.Debug("cache miss", zap.String("short_code", shortCode))
	url, err = s.urlRepo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return nil, err
	}

	// Try to cache for next time
	if err := s.cacheRepo.Set(ctx, url, s.cacheTTL); err != nil {
		s.logger.Warn("failed to cache URL", zap.Error(err))
	}

	// Track redirect for cache miss
	// Learning: Cache misses are slower (hit DB), but still count as redirects
	s.metrics.URLRedirectsTotal.Inc()

	return url, nil
}
