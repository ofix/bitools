package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CacheEntry struct {
	Data      []byte    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

type DirCache struct {
	mu      sync.RWMutex
	cache   map[string]*CacheEntry
	dataDir string
	maxAge  time.Duration
}

func NewDirCache(dataDir string) *DirCache {
	dc := &DirCache{
		cache:   make(map[string]*CacheEntry),
		dataDir: dataDir,
		maxAge:  5 * time.Minute, // 缓存 5 分钟
	}
	dc.loadFromDisk()
	return dc
}

func (dc *DirCache) Get(key string) ([]byte, bool) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	entry, ok := dc.cache[key]
	if !ok || time.Since(entry.Timestamp) > dc.maxAge {
		return nil, false
	}
	return entry.Data, true
}

func (dc *DirCache) Set(key string, data []byte) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.cache[key] = &CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
	}
	go dc.saveToDisk()
}

func (dc *DirCache) loadFromDisk() {
	path := filepath.Join(dc.dataDir, "dir_cache.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var diskCache map[string]*CacheEntry
	if err := json.Unmarshal(b, &diskCache); err == nil {
		dc.cache = diskCache
	}
}

func (dc *DirCache) saveToDisk() {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	b, err := json.Marshal(dc.cache)
	if err != nil {
		return
	}
	path := filepath.Join(dc.dataDir, "dir_cache.json")
	os.WriteFile(path, b, 0644)
}
