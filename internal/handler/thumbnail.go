package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/sashko-guz/mage/internal/logger"
	"github.com/sashko-guz/mage/internal/operations"
	"github.com/sashko-guz/mage/internal/parser"
	"github.com/sashko-guz/mage/internal/processor"
	"github.com/sashko-guz/mage/internal/signature"
	"github.com/sashko-guz/mage/internal/storage"
	storageDrivers "github.com/sashko-guz/mage/internal/storage/drivers"
	"golang.org/x/sync/singleflight"
)

type ThumbnailResult struct {
	Data        []byte
	ContentType string
}

type ThumbnailHandler struct {
	storage        storageDrivers.Storage
	processor      *processor.ImageProcessor
	singleflight   *singleflight.Group
	processSem     chan struct{}
	signer         *signature.Signature // URL signature handler
	maxInputSize   int                  // max input image size in bytes
	cachingEnabled bool                 // true if storage supports caching
}

func NewThumbnailHandler(stor storageDrivers.Storage, proc *processor.ImageProcessor, signatureCfg signature.Config, maxInputSize int) (*ThumbnailHandler, error) {
	_, cachingEnabled := stor.(*storage.CachedStorage)

	maxConcurrent := resolveMaxConcurrent()
	logger.Infof("[ThumbnailHandler] Max input image size: %d MB", maxInputSize/(1024*1024))

	signer, err := buildSigner(signatureCfg)
	if err != nil {
		return nil, err
	}

	return &ThumbnailHandler{
		storage:        stor,
		processor:      proc,
		singleflight:   &singleflight.Group{},
		processSem:     make(chan struct{}, maxConcurrent),
		signer:         signer,
		maxInputSize:   maxInputSize,
		cachingEnabled: cachingEnabled,
	}, nil
}

// resolveMaxConcurrent determines the processing concurrency limit.
// Defaults to 2x CPU cores (capped at 32) but can be overridden via VIPS_MAX_CONCURRENT.
func resolveMaxConcurrent() int {
	numCPU := runtime.NumCPU()
	maxConcurrent := min(numCPU*2, 32)

	if override := os.Getenv("VIPS_MAX_CONCURRENT"); override != "" {
		if val, err := strconv.Atoi(override); err == nil && val > 0 {
			maxConcurrent = val
			logger.Infof("[ThumbnailHandler] Using VIPS_MAX_CONCURRENT override: %d", maxConcurrent)
		}
	}

	logger.Infof("[ThumbnailHandler] Process concurrency: %d workers (CPU cores: %d)", maxConcurrent, numCPU)
	return maxConcurrent
}

// buildSigner creates the URL signature verifier if a secret key is configured.
func buildSigner(cfg signature.Config) (*signature.Signature, error) {
	if cfg.SecretKey == "" {
		return nil, nil
	}
	signer, err := signature.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid signature configuration: %w", err)
	}
	return signer, nil
}

// -------------------------------------------------------------------
// HTTP handler
// -------------------------------------------------------------------

func (h *ThumbnailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cacheKey := r.URL.Path

	if h.serveCachedThumbnail(w, cacheKey) {
		return
	}

	req, ok := h.parseRequest(w, r)
	if !ok {
		return
	}

	if !h.validateSignature(w, r, req) {
		return
	}

	h.logProcessingRequest(req, cacheKey)

	result, isDuplicate, err := h.processWithSingleflight(r, req, cacheKey)

	binaryData := h.cacheResult(cacheKey, result, err)

	if err != nil {
		h.writeError(w, r, err)
		return
	}

	thumbnail := result.(*ThumbnailResult)

	if isDuplicate {
		logger.Debugf("[ThumbnailHandler] Concurrent duplicate request served from singleflight: %s", cacheKey)
	}

	h.writeThumbnailResponse(w, thumbnail, "MISS")
	logger.Debugf("[ThumbnailHandler] Successfully generated thumbnail for: %s", req.Path)
	h.scheduleAsyncCacheWrite(cacheKey, binaryData)
}

// serveCachedThumbnail checks the thumbnail cache and writes the response if a cached entry is
// found. Returns true when the response has been served and no further processing is needed.
func (h *ThumbnailHandler) serveCachedThumbnail(w http.ResponseWriter, cacheKey string) bool {
	if !h.cachingEnabled {
		return false
	}

	cachedStore := h.storage.(*storage.CachedStorage)
	if !cachedStore.ThumbsCacheEnabled() {
		return false
	}

	cachedData, found, err := cachedStore.GetThumbnail(cacheKey)
	if err != nil || !found {
		return false
	}

	thumbnail, err := decodeThumbnailBinary(cachedData)
	if err != nil {
		logger.Warnf("[ThumbnailHandler] Error decoding cached thumbnail: %v", err)
		return false // fall through to reprocess
	}

	logger.Debugf("[ThumbnailHandler] Cache HIT - serving thumbnail immediately: %s", cacheKey)
	h.writeThumbnailResponse(w, thumbnail, "HIT")
	return true
}

// parseRequest parses the URL path and enforces signature-presence rules.
// Writes an appropriate error response and returns false on failure.
func (h *ThumbnailHandler) parseRequest(w http.ResponseWriter, r *http.Request) (*operations.Request, bool) {
	req, err := parser.ParseURL(r.URL.Path)
	if err != nil {
		logger.Warnf("[ThumbnailHandler] Error parsing URL: %v (url=%s)", err, r.URL.String())
		http.Error(w, fmt.Sprintf("Invalid URL format: %v (url=%s)", err, r.URL.String()), http.StatusBadRequest)
		return nil, false
	}

	if h.signer == nil && req.ProvidedSignature != "" {
		logger.Warnf("[ThumbnailHandler] Signed URL rejected: signature validation is disabled (url=%s)", r.URL.String())
		http.Error(w, "Signed URLs are not allowed when signature validation is disabled", http.StatusNotFound)
		return nil, false
	}

	return req, true
}

// validateSignature verifies the request signature when signing is enabled.
// Returns 404 (not 403) on failure to avoid disclosing resource existence.
func (h *ThumbnailHandler) validateSignature(w http.ResponseWriter, r *http.Request, req *operations.Request) bool {
	if h.signer == nil {
		return true
	}

	if err := h.signer.Verify(req); err != nil {
		logger.Warnf("[ThumbnailHandler] Signature validation failed: %v (url=%s)", err, r.URL.String())
		http.Error(w, fmt.Sprintf("Signature validation failed: %v", err), http.StatusNotFound)
		return false
	}

	return true
}

// logProcessingRequest emits a debug-level log describing the incoming request.
func (h *ThumbnailHandler) logProcessingRequest(req *operations.Request, urlPath string) {
	if !logger.EnabledDebug() {
		return
	}

	prefix := ""
	if h.cachingEnabled {
		prefix = "Cache MISS - "
	}

	sigInfo := ""
	if req.ProvidedSignature != "" {
		sigInfo = fmt.Sprintf(", signature=%s", req.ProvidedSignature)
	}

	logger.Debugf("[ThumbnailHandler] %sProcessing thumbnail: path=%s, operations=[%s]%s, url=%s",
		prefix, req.Path, formatOperations(req.Operations, req.FilterString), sigInfo, urlPath)
}

// processWithSingleflight executes thumbnail generation under singleflight deduplication,
// so concurrent identical requests share a single in-flight computation.
func (h *ThumbnailHandler) processWithSingleflight(r *http.Request, req *operations.Request, cacheKey string) (any, bool, error) {
	result, err, isDuplicate := h.singleflight.Do(cacheKey, func() (any, error) {
		h.processSem <- struct{}{}
		defer func() { <-h.processSem }()
		return h.fetchAndProcess(r, req)
	})
	return result, isDuplicate, err
}

// fetchAndProcess fetches the source image from storage and generates the thumbnail.
func (h *ThumbnailHandler) fetchAndProcess(r *http.Request, req *operations.Request) (*ThumbnailResult, error) {
	imageData, err := h.storage.GetObject(r.Context(), req.Path)
	if err != nil {
		logger.Errorf("[ThumbnailHandler] Error fetching image from storage: %v", err)
		return nil, err
	}

	if h.maxInputSize > 0 && len(imageData) > h.maxInputSize {
		logger.Warnf("[ThumbnailHandler] Source image too large: path=%s, size=%d bytes, limit=%d bytes",
			req.Path, len(imageData), h.maxInputSize)
		return nil, &inputImageTooLargeError{Actual: len(imageData), Limit: h.maxInputSize}
	}

	thumbnail, contentType, err := h.processor.CreateThumbnail(imageData, req)
	if err != nil {
		logger.Errorf("[ThumbnailHandler] Error creating thumbnail: %v", err)
		return nil, err
	}

	return &ThumbnailResult{Data: thumbnail, ContentType: contentType}, nil
}

// cacheResult stores the thumbnail in the synchronous (memory) cache and returns the encoded
// binary data so the caller can schedule an async disk write afterwards.
func (h *ThumbnailHandler) cacheResult(cacheKey string, result any, err error) []byte {
	if !h.cachingEnabled || err != nil || result == nil {
		return nil
	}

	cachedStore := h.storage.(*storage.CachedStorage)
	if !cachedStore.ThumbsCacheEnabled() {
		return nil
	}

	binaryData := encodeThumbnailBinary(result.(*ThumbnailResult))
	if cacheErr := cachedStore.SetThumbnail(cacheKey, binaryData); cacheErr != nil {
		logger.Warnf("[ThumbnailHandler] Error caching thumbnail result: %v", cacheErr)
	}

	return binaryData
}

// writeError translates a processing error into the appropriate HTTP error response.
func (h *ThumbnailHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	if tooLargeErr, ok := errors.AsType[*inputImageTooLargeError](err); ok {
		http.Error(w,
			fmt.Sprintf("Source image is too large: %.2f MB exceeds limit %.2f MB",
				float64(tooLargeErr.Actual)/(1024*1024),
				float64(tooLargeErr.Limit)/(1024*1024),
			),
			http.StatusRequestEntityTooLarge,
		)
		return
	}

	http.Error(w,
		fmt.Sprintf("Failed to create thumbnail: %v (url=%s)", err, r.URL.String()),
		http.StatusInternalServerError,
	)
}

// writeThumbnailResponse sends the thumbnail bytes with standard caching headers.
// cacheStatus is used as the X-Cache header value ("HIT" or "MISS").
func (h *ThumbnailHandler) writeThumbnailResponse(w http.ResponseWriter, thumbnail *ThumbnailResult, cacheStatus string) {
	w.Header().Set("Content-Type", thumbnail.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year
	w.Header().Set("X-Cache", cacheStatus)
	w.Header().Set("Content-Length", strconv.Itoa(len(thumbnail.Data)))

	if _, err := w.Write(thumbnail.Data); err != nil {
		logger.Warnf("[ThumbnailHandler] Error writing response: %v", err)
	}
}

// scheduleAsyncCacheWrite queues a background disk-cache write after the response is sent.
func (h *ThumbnailHandler) scheduleAsyncCacheWrite(cacheKey string, binaryData []byte) {
	if binaryData == nil {
		return
	}
	cachedStore := h.storage.(*storage.CachedStorage)
	cachedStore.SetThumbnailAsync(cacheKey, binaryData)
}

// -------------------------------------------------------------------
// Binary encoding helpers
// -------------------------------------------------------------------

// encodeThumbnailBinary encodes a ThumbnailResult to a compact binary format.
// Layout: [4 bytes: content-type length (big-endian)][content-type bytes][image data]
func encodeThumbnailBinary(t *ThumbnailResult) []byte {
	ctLen := len(t.ContentType)
	out := make([]byte, 4+ctLen+len(t.Data))

	out[0] = byte(ctLen >> 24)
	out[1] = byte(ctLen >> 16)
	out[2] = byte(ctLen >> 8)
	out[3] = byte(ctLen)

	copy(out[4:], t.ContentType)
	copy(out[4+ctLen:], t.Data)

	return out
}

// decodeThumbnailBinary decodes a binary-encoded thumbnail back to a ThumbnailResult.
func decodeThumbnailBinary(data []byte) (*ThumbnailResult, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("invalid binary thumbnail format: too short")
	}

	ctLen := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	if len(data) < 4+int(ctLen) {
		return nil, fmt.Errorf("invalid binary thumbnail format: content type truncated")
	}

	return &ThumbnailResult{
		ContentType: string(data[4 : 4+ctLen]),
		Data:        data[4+ctLen:],
	}, nil
}

// -------------------------------------------------------------------
// Operation formatting (for debug logging)
// -------------------------------------------------------------------

// formatOperations returns a human-readable summary of the operation pipeline.
func formatOperations(ops []operations.Operation, filterString string) string {
	if len(ops) == 0 {
		return "none"
	}

	hasFit := containsFitOperation(ops)
	hasExplicitQuality := strings.Contains(filterString, "quality(")
	defaultQuality := findQualityValue(ops)

	var parts []string
	for _, op := range ops {
		if s, ok := formatOperation(op, hasFit, hasExplicitQuality, defaultQuality); ok {
			parts = append(parts, s)
		}
	}

	return strings.Join(parts, " → ")
}

func containsFitOperation(ops []operations.Operation) bool {
	for _, op := range ops {
		if _, ok := op.(*operations.FitOperation); ok {
			return true
		}
	}
	return false
}

func findQualityValue(ops []operations.Operation) int {
	for _, op := range ops {
		if q, ok := op.(*operations.QualityOperation); ok {
			return q.Quality
		}
	}
	return 0
}

// formatOperation formats a single operation for logging.
// Returns ("", false) when the operation should be omitted from the output.
func formatOperation(op operations.Operation, hasFit, hasExplicitQuality bool, defaultQuality int) (string, bool) {
	switch v := op.(type) {
	case *operations.ResizeOperation:
		return formatResizeOperation(v, hasFit, hasExplicitQuality, defaultQuality), true
	case *operations.FormatOperation:
		return fmt.Sprintf("format(%s)", v.Format), true
	case *operations.QualityOperation:
		if hasExplicitQuality {
			return fmt.Sprintf("quality(%d)", v.Quality), true
		}
		return "", false // default quality is shown inline with resize
	case *operations.CropOperation:
		return fmt.Sprintf("crop(%d,%d,%d,%d)", v.X1, v.Y1, v.X2, v.Y2), true
	case *operations.PercentCropOperation:
		return fmt.Sprintf("pcrop(%d,%d,%d,%d)", v.X1, v.Y1, v.X2, v.Y2), true
	case *operations.FitOperation:
		return formatFitOperation(v), true
	default:
		return op.Name(), true
	}
}

func formatResizeOperation(v *operations.ResizeOperation, hasFit, hasExplicitQuality bool, defaultQuality int) string {
	widthStr, heightStr := "auto", "auto"
	if v.Width != nil {
		widthStr = strconv.Itoa(*v.Width)
	}
	if v.Height != nil {
		heightStr = strconv.Itoa(*v.Height)
	}

	s := fmt.Sprintf("resize(%sx%s", widthStr, heightStr)
	if !hasFit {
		s += fmt.Sprintf(", fit=%s", v.Fit)
	}
	if !hasExplicitQuality {
		s += fmt.Sprintf(", quality(%d)", defaultQuality)
	}
	return s + ")"
}

func formatFitOperation(v *operations.FitOperation) string {
	if v.FillColor != "" && v.Mode == "fill" {
		return fmt.Sprintf("fit(%s, %s)", v.Mode, v.FillColor)
	}
	return fmt.Sprintf("fit(%s)", v.Mode)
}

// -------------------------------------------------------------------
// Error types
// -------------------------------------------------------------------

type inputImageTooLargeError struct {
	Actual int
	Limit  int
}

func (e *inputImageTooLargeError) Error() string {
	return fmt.Sprintf("source image size %d bytes exceeds configured limit %d bytes", e.Actual, e.Limit)
}
