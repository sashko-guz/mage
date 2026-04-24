package disk

import (
	"fmt"

	"github.com/hashicorp/golang-lru/v2/simplelru"
)

// -------------------------------------------------------------------
// LRU management
// -------------------------------------------------------------------

func (dc *DiskCache) initLRU() {
	lruIndex, err := simplelru.NewLRU(dc.MaxItems, func(_ string, entry *cacheEntry) {
		if entry == nil {
			return
		}
		dc.currentSize.Add(-entry.size)
		dc.enqueueDelete(entry.path)
	})
	if err != nil {
		panic(fmt.Sprintf("failed to initialize LRU index: %v", err))
	}
	dc.lru = lruIndex
}

// updateLRUEntry registers or replaces an entry in the LRU index, then evicts if over the size limit.
func (dc *DiskCache) updateLRUEntry(entry *cacheEntry) {
	dc.mu.Lock()
	if _, exists := dc.lru.Peek(entry.hash); exists {
		dc.lru.Remove(entry.hash)
	}
	dc.currentSize.Add(entry.size)
	dc.lru.Add(entry.hash, entry)
	dc.evictForSizeLocked(2048)
	dc.mu.Unlock()
}

// removeStaleLRUEntry removes the LRU entry for hash if it still points to the given path.
func (dc *DiskCache) removeStaleLRUEntry(hash, path string) {
	dc.mu.Lock()
	if stale, exists := dc.lru.Peek(hash); exists && stale.path == path {
		dc.lru.Remove(hash)
	}
	dc.mu.Unlock()
}

// evictForSizeLocked evicts LRU entries until the cache is below the low-water mark.
// Must be called with dc.mu held.
func (dc *DiskCache) evictForSizeLocked(maxRemovals int) int {
	high, low := dc.sizeWatermarks()
	if high <= 0 || dc.currentSize.Load() <= high {
		return 0
	}

	removed := 0
	for dc.currentSize.Load() > low {
		if maxRemovals > 0 && removed >= maxRemovals {
			break
		}
		_, _, ok := dc.lru.RemoveOldest()
		if !ok {
			break
		}
		removed++
	}
	return removed
}

func (dc *DiskCache) sizeWatermarks() (high int64, low int64) {
	if dc.MaxSize <= 0 {
		return 0, 0
	}

	high = (dc.MaxSize * 95) / 100
	low = (dc.MaxSize * 85) / 100

	if high <= 0 {
		high = dc.MaxSize
	}
	if low <= 0 || low >= high {
		low = (high * 9) / 10
		if low <= 0 {
			low = high
		}
	}
	return high, low
}
