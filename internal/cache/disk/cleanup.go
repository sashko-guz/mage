package disk

import (
	"os"
	"time"

	"github.com/sashko-guz/mage/internal/format"
	"github.com/sashko-guz/mage/internal/logger"
)

// -------------------------------------------------------------------
// Cleanup goroutine
// -------------------------------------------------------------------

// cleanupExpired periodically removes expired cache entries.
func (dc *DiskCache) cleanupExpired() {
	const (
		baseInterval      = 30 * time.Second
		maxInterval       = 10 * time.Minute
		idleStep          = 30 * time.Second
		missingCheckEvery = 10
	)

	interval := baseInterval
	runs := 0
	timer := time.NewTimer(interval)
	defer timer.Stop()

	logger.Debugf("[DiskCache] Cleanup goroutine started for %s, base interval %v", dc.basePath, baseInterval)

	for {
		select {
		case <-timer.C:
		case <-dc.cleanupWake:
			if interval > baseInterval {
				stats := dc.performCleanup(false)
				interval = baseInterval
				resetTimer(timer, interval)
				logger.Debugf("[DiskCache] Activity detected, cleanup accelerated (removed=%d, entries=%d, size=%v)",
					stats.removed, stats.currentCount, format.Bytes(stats.currentSize))
			}
			continue
		}

		runs++
		checkMissingFiles := runs%missingCheckEvery == 0
		stats := dc.performCleanup(checkMissingFiles)
		interval = nextCleanupInterval(stats, interval, baseInterval, maxInterval, idleStep)
		timer.Reset(interval)
		logger.Debugf("[DiskCache] Cleanup scheduled in directory %s with interval %v (removed=%d, entries=%d, size=%v)",
			dc.basePath, interval, stats.removed, stats.currentCount, format.Bytes(stats.currentSize))
	}
}

// nextCleanupInterval backs off toward maxInterval when the cache is idle,
// and resets to baseInterval when entries are being actively removed.
func nextCleanupInterval(stats cleanupStats, current, base, max, step time.Duration) time.Duration {
	if stats.currentCount == 0 || stats.removed == 0 {
		if next := current + step; next < max {
			return next
		}
		return max
	}
	return base
}

// resetTimer safely stops and resets a timer, draining any pending tick.
func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

func (dc *DiskCache) notifyActivity() {
	select {
	case dc.cleanupWake <- struct{}{}:
	default:
	}
}

// -------------------------------------------------------------------
// Cleanup implementation
// -------------------------------------------------------------------

func (dc *DiskCache) performCleanup(checkMissingFiles bool) cleanupStats {
	now := time.Now()

	expired, existing := dc.scanCandidates(now, checkMissingFiles)

	var missing []cleanupCandidate
	if checkMissingFiles {
		missing = dc.findMissingFiles(existing)
	}

	removed, size, count := dc.pruneIndex(expired, missing, now)
	return cleanupStats{removed: removed, currentSize: size, currentCount: count}
}

// scanCandidates walks a portion of the LRU index under the lock and returns:
//   - expired: entries whose TTL has elapsed
//   - existing: non-expired entries to be checked for missing files (only when collectExisting is true)
func (dc *DiskCache) scanCandidates(now time.Time, collectExisting bool) (expired, existing []cleanupCandidate) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	keys := dc.lru.Keys()
	keyCount := len(keys)
	if keyCount == 0 {
		dc.cleanupPos = 0
		return
	}

	start := dc.cleanupPos % keyCount
	scanCount := min(cleanupScanBudget, keyCount)

	for i := range scanCount {
		key := keys[(start+i)%keyCount]
		entry, ok := dc.lru.Peek(key)
		if !ok || entry == nil {
			continue
		}

		c := cleanupCandidate{key: key, path: entry.path}
		if now.After(entry.expiresAt) {
			expired = append(expired, c)
			if len(expired) >= cleanupRemoveBudget {
				break
			}
		} else if collectExisting && len(existing) < cleanupRemoveBudget {
			existing = append(existing, c)
		}
	}

	dc.cleanupPos = (start + scanCount) % keyCount
	return
}

// findMissingFiles returns entries from candidates whose backing file no longer exists on disk.
func (dc *DiskCache) findMissingFiles(candidates []cleanupCandidate) []cleanupCandidate {
	var missing []cleanupCandidate
	for _, c := range candidates {
		if _, err := os.Stat(c.path); os.IsNotExist(err) {
			missing = append(missing, c)
			if len(missing) >= cleanupRemoveBudget {
				break
			}
		}
	}
	return missing
}

// pruneIndex removes expired and missing entries from the LRU index under the lock,
// then evicts for size. Returns removal count, current cache size, and entry count.
func (dc *DiskCache) pruneIndex(expired, missing []cleanupCandidate, now time.Time) (removed int, size int64, count int) {
	dc.mu.Lock()

	for _, c := range expired {
		if entry, ok := dc.lru.Peek(c.key); ok && entry != nil && entry.path == c.path && now.After(entry.expiresAt) {
			dc.lru.Remove(c.key)
			removed++
		}
	}
	for _, c := range missing {
		if entry, ok := dc.lru.Peek(c.key); ok && entry != nil && entry.path == c.path {
			dc.lru.Remove(c.key)
			removed++
		}
	}

	removed += dc.evictForSizeLocked(1024)
	size = dc.currentSize.Load()
	count = dc.lru.Len()
	dc.mu.Unlock()
	return
}
