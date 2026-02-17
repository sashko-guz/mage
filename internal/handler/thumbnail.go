package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/sashko-guz/mage/internal/logger"
	"github.com/sashko-guz/mage/internal/operations"
	"github.com/sashko-guz/mage/internal/parser"
	"github.com/sashko-guz/mage/internal/processor"
	"github.com/sashko-guz/mage/internal/storage"
	"golang.org/x/sync/singleflight"
)

type ThumbnailResult struct {
	Data        []byte
	ContentType string
}

type ThumbnailHandler struct {
	storage        storage.Storage
	processor      *processor.ImageProcessor
	singleflight   *singleflight.Group
	processSem     chan struct{}
	signer         *parser.Signature // URL signature handler
	cachingEnabled bool              // true if storage supports caching
}

func NewThumbnailHandler(stor storage.Storage, processor *processor.ImageProcessor, signatureKey string) (*ThumbnailHandler, error) {
	// Check once at startup if caching is enabled
	_, cachingEnabled := stor.(*storage.CachedStorage)

	// Determine optimal concurrency based on CPU cores
	// Rule: 2x CPU cores for I/O + CPU bound work, capped at 32 to prevent memory exhaustion
	numCPU := runtime.NumCPU()
	maxConcurrent := min(numCPU*2, 32)

	// Allow override via environment variable
	if override := os.Getenv("VIPS_MAX_CONCURRENT"); override != "" {
		if val, err := strconv.Atoi(override); err == nil && val > 0 {
			maxConcurrent = val
			logger.Infof("[ThumbnailHandler] Using VIPS_MAX_CONCURRENT override: %d", maxConcurrent)
		}
	}

	logger.Infof("[ThumbnailHandler] Process concurrency: %d workers (CPU cores: %d)", maxConcurrent, numCPU)

	return &ThumbnailHandler{
		storage:        stor,
		processor:      processor,
		singleflight:   &singleflight.Group{},
		processSem:     make(chan struct{}, maxConcurrent), // Dynamic based on CPU
		signer:         parser.NewSignature(signatureKey),
		cachingEnabled: cachingEnabled,
	}, nil
}

// Buffer pool for reducing allocations in encodeThumbnailBinary
// Pre-allocate 256KB buffers which covers most thumbnail sizes
var bufferPool = sync.Pool{
	New: func() any {
		// Pre-allocate 256KB buffers (covers most thumbnails)
		return bytes.NewBuffer(make([]byte, 0, 256*1024))
	},
}

// encodeThumbnailBinary encodes ThumbnailResult to binary format
// Format: [4 bytes: content-type length][content-type][image data]
// This avoids expensive JSON marshaling/unmarshaling of large binary data
// Uses buffer pooling to reduce allocations and GC pressure
func encodeThumbnailBinary(t *ThumbnailResult) []byte {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()               // Clear previous data
	defer bufferPool.Put(buf) // Return to pool

	// Write content type length (4 bytes, big-endian)
	ctLen := uint32(len(t.ContentType))
	buf.WriteByte(byte(ctLen >> 24))
	buf.WriteByte(byte(ctLen >> 16))
	buf.WriteByte(byte(ctLen >> 8))
	buf.WriteByte(byte(ctLen))

	// Write content type
	buf.WriteString(t.ContentType)

	// Write image data
	buf.Write(t.Data)

	// Copy to new slice since buffer will be reused
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return result
}

// decodeThumbnailBinary decodes binary format back to ThumbnailResult
func decodeThumbnailBinary(data []byte) (*ThumbnailResult, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("invalid binary thumbnail format: too short")
	}

	// Read content type length
	ctLen := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])

	if len(data) < 4+int(ctLen) {
		return nil, fmt.Errorf("invalid binary thumbnail format: content type truncated")
	}

	// Read content type
	contentType := string(data[4 : 4+ctLen])

	// Read image data
	imageData := data[4+ctLen:]

	return &ThumbnailResult{
		Data:        imageData,
		ContentType: contentType,
	}, nil
}

// formatOperations formats the operations list for logging
func formatOperations(ops []operations.Operation, filterString string) string {
	if len(ops) == 0 {
		return "none"
	}

	// Check if there's an explicit FitOperation in the list
	hasFitOperation := false
	for _, op := range ops {
		if _, ok := op.(*operations.FitOperation); ok {
			hasFitOperation = true
			break
		}
	}

	// Check if there's an explicit QualityOperation in the filters
	hasExplicitQuality := strings.Contains(filterString, "quality(")

	// Get the quality value for default display
	var qualityValue int
	for _, op := range ops {
		if qualOp, ok := op.(*operations.QualityOperation); ok {
			qualityValue = qualOp.Quality
			break
		}
	}

	var parts []string
	for _, op := range ops {
		switch v := op.(type) {
		case *operations.ResizeOperation:
			widthStr := "auto"
			heightStr := "auto"
			if v.Width != nil {
				widthStr = strconv.Itoa(*v.Width)
			}
			if v.Height != nil {
				heightStr = strconv.Itoa(*v.Height)
			}
			// Build resize string with fit mode if explicit, and quality if default
			resizeStr := fmt.Sprintf("resize(%sx%s", widthStr, heightStr)
			if !hasFitOperation {
				resizeStr += fmt.Sprintf(", fit=%s", v.Fit)
			}
			if !hasExplicitQuality {
				// Quality is default, show it inline with resize
				resizeStr += fmt.Sprintf(", quality(%d)", qualityValue)
			}
			resizeStr += ")"
			parts = append(parts, resizeStr)
		case *operations.FormatOperation:
			parts = append(parts, fmt.Sprintf("format(%s)", v.Format))
		case *operations.QualityOperation:
			// Only show quality if it was explicitly specified in filters
			if hasExplicitQuality {
				parts = append(parts, fmt.Sprintf("quality(%d)", v.Quality))
			}
		case *operations.CropOperation:
			parts = append(parts, fmt.Sprintf("crop(%d,%d,%d,%d)", v.X1, v.Y1, v.X2, v.Y2))
		case *operations.PercentCropOperation:
			parts = append(parts, fmt.Sprintf("pcrop(%d,%d,%d,%d)", v.X1, v.Y1, v.X2, v.Y2))
		case *operations.FitOperation:
			if v.FillColor != "" && v.Mode == "fill" {
				parts = append(parts, fmt.Sprintf("fit(%s, %s)", v.Mode, v.FillColor))
			} else {
				parts = append(parts, fmt.Sprintf("fit(%s)", v.Mode))
			}
		default:
			parts = append(parts, op.Name())
		}
	}

	return strings.Join(parts, " → ")
}

func (h *ThumbnailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	store := h.storage
	cacheKey := r.URL.Path

	// PERFORMANCE OPTIMIZATION: Check cache FIRST before parsing (if enabled)
	// This allows cache hits to bypass expensive URL parsing, operation parsing, and signature validation
	if h.cachingEnabled {
		cachedStore := store.(*storage.CachedStorage)

		// Only check thumbnail cache if it's actually enabled
		if cachedStore.ThumbsCacheEnabled() {
			// Check thumbnail cache first (memory → disk)
			if cachedData, found, err := cachedStore.GetThumbnail(cacheKey); err == nil && found {
				logger.Debugf("[ThumbnailHandler] Cache HIT - serving thumbnail immediately: %s", cacheKey)

				// Decode cached result using binary format (no JSON overhead)
				thumbnail, err := decodeThumbnailBinary(cachedData)
				if err != nil {
					logger.Warnf("[ThumbnailHandler] Error decoding cached thumbnail: %v", err)
					// Continue to reprocess if cache is corrupted
				} else {
					// Send cached response - no parsing or signature validation needed!
					w.Header().Set("Content-Type", thumbnail.ContentType)
					w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
					w.Header().Set("X-Cache", "HIT")
					w.Header().Set("Content-Length", strconv.Itoa(len(thumbnail.Data)))

					if _, err := w.Write(thumbnail.Data); err != nil {
						logger.Warnf("[ThumbnailHandler] Error writing response: %v", err)
					}
					return
				}
			}
		}
	}

	// Parse URL and process request
	// Parse URL path: /thumbs/[{signature}/]{size}/[filters:{filters}/]{path}
	req, err := parser.ParseURL(r.URL.Path, "")
	if err != nil {
		logger.Warnf("[ThumbnailHandler] Error parsing URL: %v (url=%s)", err, r.URL.String())
		http.Error(w, fmt.Sprintf("Invalid URL format: %v (url=%s)", err, r.URL.String()), http.StatusBadRequest)
		return
	}

	if logger.EnabledDebug() {
		sigInfo := ""
		if req.ProvidedSignature != "" {
			sigInfo = fmt.Sprintf(", signature=%s", req.ProvidedSignature)
		}
		cacheMsgPrefix := ""
		if h.cachingEnabled {
			cacheMsgPrefix = "Cache MISS - "
		}
		logger.Debugf("[ThumbnailHandler] %sProcessing thumbnail: path=%s, operations=[%s]%s, url=%s",
			cacheMsgPrefix, req.Path, formatOperations(req.Operations, req.FilterString), sigInfo, r.URL.Path)
	}

	// Verify signature if enabled
	if err := h.signer.Verify(req); err != nil {
		logger.Warnf("[ThumbnailHandler] Signature validation failed: %v (url=%s)", err, r.URL.String())
		// Return 404 instead of 403 to avoid information disclosure
		// This prevents revealing whether a resource exists when signature is invalid
		http.Error(w, fmt.Sprintf("Signature validation failed: %v", err), http.StatusNotFound)
		return
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
			logger.Errorf("[ThumbnailHandler] Error fetching image from storage: %v", err)
			return nil, err
		}

		// Generate thumbnail
		thumbnail, contentType, err := h.processor.CreateThumbnail(imageData, req)
		if err != nil {
			logger.Errorf("[ThumbnailHandler] Error creating thumbnail: %v", err)
			return nil, err
		}

		return &ThumbnailResult{
			Data:        thumbnail,
			ContentType: contentType,
		}, nil
	})

	// Cache the thumbnail result if thumbnail caching is enabled
	// SetThumbnail: Memory cache synchronously (fast, user sees response immediately)
	// SetThumbnailAsync: Disk cache asynchronously (background, doesn't block response)
	if h.cachingEnabled && err == nil && result != nil {
		cachedStore := store.(*storage.CachedStorage)

		// Only cache if thumbnails caching is actually enabled
		if cachedStore.ThumbsCacheEnabled() {
			thumbnail := result.(*ThumbnailResult)
			// Encode to binary format (avoids JSON overhead)
			binaryData := encodeThumbnailBinary(thumbnail)

			// Store in memory cache synchronously (fast)
			if cacheErr := cachedStore.SetThumbnail(cacheKey, binaryData); cacheErr != nil {
				logger.Warnf("[ThumbnailHandler] Error caching thumbnail result: %v", cacheErr)
				// Don't fail if caching fails, just continue
			}
		}
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create thumbnail: %v (url=%s)", err, r.URL.String()), http.StatusInternalServerError)
		return
	}

	thumbnail := result.(*ThumbnailResult)

	// Log if this was a concurrent duplicate request
	if isDuplicate {
		logger.Debugf("[ThumbnailHandler] Concurrent duplicate request served from singleflight: %s", cacheKey)
	}

	// Send response
	w.Header().Set("Content-Type", thumbnail.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
	w.Header().Set("X-Cache", "MISS")
	w.Header().Set("Content-Length", strconv.Itoa(len(thumbnail.Data)))

	if _, err := w.Write(thumbnail.Data); err != nil {
		logger.Warnf("[ThumbnailHandler] Error writing response: %v", err)
	}

	logger.Debugf("[ThumbnailHandler] Successfully generated thumbnail for: %s", req.Path)

	// Queue asynchronous disk cache write (happens in background after response is sent)
	if h.cachingEnabled && result != nil {
		cachedStore := store.(*storage.CachedStorage)
		if cachedStore.ThumbsCacheEnabled() {
			thumbnail := result.(*ThumbnailResult)
			binaryData := encodeThumbnailBinary(thumbnail)
			// This is non-blocking and returns immediately
			cachedStore.SetThumbnailAsync(cacheKey, binaryData)
		}
	}
}
