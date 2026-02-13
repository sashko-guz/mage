package handler

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/sashko-guz/mage/internal/parser"
	"github.com/sashko-guz/mage/internal/processor"
	"github.com/sashko-guz/mage/internal/storage"
	"golang.org/x/sync/singleflight"
)

type ThumbnailResult struct {
	Data        []byte `json:"data"`
	ContentType string `json:"content_type"`
}

type ThumbnailHandler struct {
	storage      storage.Storage
	processor    *processor.ImageProcessor
	singleflight *singleflight.Group
	processSem   chan struct{}
	signer       *parser.Signature // URL signature handler
}

func NewThumbnailHandler(stor storage.Storage, processor *processor.ImageProcessor, signatureKey string) (*ThumbnailHandler, error) {
	return &ThumbnailHandler{
		storage:      stor,
		processor:    processor,
		singleflight: &singleflight.Group{},
		processSem:   make(chan struct{}, 16), // limit concurrent vips work to avoid crash
		signer:       parser.NewSignature(signatureKey),
	}, nil
}

func (h *ThumbnailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse URL path: /thumbs/[{signature}/]{size}/[filters:{filters}/]{path}
	req, err := parser.ParseURL(r.URL.Path, "")
	if err != nil {
		log.Printf("[ThumbnailHandler] Error parsing URL: %v (url=%s)", err, r.URL.String())
		http.Error(w, fmt.Sprintf("Invalid URL format: %v (url=%s)", err, r.URL.String()), http.StatusBadRequest)
		return
	}

	// Format size for logging
	var sizeStr string
	if req.Width == nil && req.Height == nil {
		sizeStr = "original"
	} else if req.Width == nil {
		sizeStr = fmt.Sprintf("x%d", *req.Height)
	} else if req.Height == nil {
		sizeStr = fmt.Sprintf("%dx", *req.Width)
	} else {
		sizeStr = fmt.Sprintf("%dx%d", *req.Width, *req.Height)
	}

	log.Printf("[ThumbnailHandler] Processing thumbnail: path=%s, size=%s, format=%s, quality=%d, fit=%s, fillColor=%s",
		req.Path, sizeStr, req.Format, req.Quality, req.Fit, req.FillColor)

	// Verify signature if enabled
	if err := h.signer.Verify(req); err != nil {
		log.Printf("[ThumbnailHandler] Signature validation failed: %v (url=%s)", err, r.URL.String())
		// Return 404 instead of 403 to avoid information disclosure
		// This prevents revealing whether a resource exists when signature is invalid
		http.Error(w, fmt.Sprintf("Signature validation failed: %v", err), http.StatusNotFound)
		return
	}

	store := h.storage

	// Use URL path as cache key for thumbnails
	cacheKey := r.URL.Path

	// Check if storage supports caching (unified cache for sources and thumbnails)
	cachedStore, hasCaching := store.(*storage.CachedStorage)
	if hasCaching {
		// Check thumbnail cache first (memory → disk → generate)
		if cachedData, found, err := cachedStore.GetThumbnail(cacheKey); err == nil && found {
			log.Printf("[ThumbnailHandler] Serving thumbnail from cache: %s", cacheKey)

			// Unmarshal the cached result
			var thumbnail ThumbnailResult
			if err := json.Unmarshal(cachedData, &thumbnail); err != nil {
				log.Printf("[ThumbnailHandler] Error unmarshaling cached thumbnail: %v", err)
				// Continue to reprocess if cache is corrupted
			} else {
				// Send cached response
				w.Header().Set("Content-Type", thumbnail.ContentType)
				w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
				w.Header().Set("X-Cache", "HIT")

				// Check if client accepts gzip
				if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
					w.Header().Set("Content-Encoding", "gzip")
					gz := gzip.NewWriter(w)
					defer gz.Close()
					if _, err := gz.Write(thumbnail.Data); err != nil {
						log.Printf("[ThumbnailHandler] Error writing gzip response: %v", err)
					}
				} else {
					w.Header().Set("Content-Length", strconv.Itoa(len(thumbnail.Data)))
					if _, err := w.Write(thumbnail.Data); err != nil {
						log.Printf("[ThumbnailHandler] Error writing response: %v", err)
					}
				}
				return
			}
		}
	}

	// Use singleflight to deduplicate concurrent identical requests
	// Do or wait for result (returns value, error, shared bool)
	result, err, isDuplicate := h.singleflight.Do(cacheKey, func() (any, error) {
		// Limit concurrent processing to reduce CPU/memory thrash under load
		h.processSem <- struct{}{}
		defer func() { <-h.processSem }()

		// Fetch image from storage (with source caching)
		imageData, err := store.GetObject(r.Context(), req.Path)
		if err != nil {
			log.Printf("[ThumbnailHandler] Error fetching image from storage: %v", err)
			return nil, err
		}

		// Generate thumbnail
		thumbnail, contentType, err := h.processor.CreateThumbnail(imageData, &processor.ThumbnailOptions{
			Width:     req.Width,
			Height:    req.Height,
			Format:    req.Format,
			Quality:   req.Quality,
			Fit:       req.Fit,
			FillColor: req.FillColor,
		})
		if err != nil {
			log.Printf("[ThumbnailHandler] Error creating thumbnail: %v", err)
			return nil, err
		}

		return &ThumbnailResult{
			Data:        thumbnail,
			ContentType: contentType,
		}, nil
	})

	// Cache the thumbnail result if caching is enabled
	if hasCaching && err == nil && result != nil {
		thumbnail := result.(*ThumbnailResult)

		// Marshal to JSON and store in unified cache
		if jsonData, marshalErr := json.Marshal(thumbnail); marshalErr == nil {
			if cacheErr := cachedStore.SetThumbnail(cacheKey, jsonData); cacheErr != nil {
				log.Printf("[ThumbnailHandler] Error caching thumbnail result: %v", cacheErr)
				// Don't fail if caching fails, just continue
			}
		} else {
			log.Printf("[ThumbnailHandler] Error marshaling thumbnail for cache: %v", marshalErr)
		}
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create thumbnail: %v (url=%s)", err, r.URL.String()), http.StatusInternalServerError)
		return
	}

	thumbnail := result.(*ThumbnailResult)

	// Log if this was a concurrent duplicate request
	if isDuplicate {
		log.Printf("[ThumbnailHandler] Concurrent duplicate request served from singleflight: %s", cacheKey)
	}

	// Send response
	w.Header().Set("Content-Type", thumbnail.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
	w.Header().Set("X-Cache", "MISS")

	// Check if client accepts gzip
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		if _, err := gz.Write(thumbnail.Data); err != nil {
			log.Printf("[ThumbnailHandler] Error writing gzip response: %v", err)
		}
	} else {
		w.Header().Set("Content-Length", strconv.Itoa(len(thumbnail.Data)))
		if _, err := w.Write(thumbnail.Data); err != nil {
			log.Printf("[ThumbnailHandler] Error writing response: %v", err)
		}
	}

	log.Printf("[ThumbnailHandler] Successfully generated thumbnail for: %s", req.Path)
}
