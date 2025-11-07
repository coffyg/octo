package octo

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// StaticConfig holds configuration for static file serving
type StaticConfig struct {
	Root           string
	Index          string
	Browse         bool
	MaxAge         int
	EnableCaching  bool
	CacheMaxSize   int64
	CacheMaxFiles  int
}

// staticCache holds cached file information
type staticCache struct {
	// Use sync.Map for lock-free reads in common case
	files    sync.Map // map[string]*cachedFile
	size     atomic.Int64
	maxSize  int64
	maxFiles int
	// Separate mutex only for eviction
	evictMu  sync.Mutex
}

type cachedFile struct {
	content  []byte
	modTime  time.Time
	size     int64
	etag     string
	lastUsed time.Time
}

var (
	// Global static file cache
	fileCache = &staticCache{
		files:    sync.Map{},
		maxSize:  100 * 1024 * 1024, // 100MB default
		maxFiles: 1000,
	}
)

// Static creates a handler for serving static files
func Static[V any](urlPrefix string, config StaticConfig) HandlerFunc[V] {
	// Ensure urlPrefix ends with /
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}

	// Set defaults
	if config.Index == "" {
		config.Index = "index.html"
	}
	if config.CacheMaxSize > 0 {
		fileCache.maxSize = config.CacheMaxSize
	}
	if config.CacheMaxFiles > 0 {
		fileCache.maxFiles = config.CacheMaxFiles
	}

	return func(ctx *Ctx[V]) {
		// Get the file path from URL
		path := ctx.Request.URL.Path
		if !strings.HasPrefix(path, urlPrefix) {
			ctx.Send404()
			return
		}

		// Get wildcard parameter
		wildcardParam := "filepath"
		if strings.Contains(urlPrefix, "*") {
			// Extract wildcard parameter name
			parts := strings.Split(urlPrefix, "*")
			if len(parts) == 2 {
				wildcardParam = parts[1]
			}
			urlPrefix = strings.TrimSuffix(urlPrefix, "*"+wildcardParam)
		}
		
		// Get file path from parameters or URL
		filePath := ""
		if fp, ok := ctx.Params[wildcardParam]; ok {
			filePath = fp
		} else {
			// Fallback to URL path parsing
			filePath = strings.TrimPrefix(path, urlPrefix)
		}
		
		if filePath == "" {
			filePath = config.Index
		}

		// Clean and validate path
		filePath = filepath.Clean(filePath)
		if strings.Contains(filePath, "..") {
			ctx.SendError("err_invalid_request", New(ErrInvalidRequest, "Invalid file path"))
			return
		}

		// Full file system path
		fullPath := filepath.Join(config.Root, filePath)

		// Check if path is a directory
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				ctx.Send404()
				return
			}
			ctx.SendError("err_internal_error", Wrap(err, ErrInternal, "Failed to stat file"))
			return
		}

		// Handle directory
		if info.IsDir() {
			// Try index file
			indexPath := filepath.Join(fullPath, config.Index)
			if indexInfo, err := os.Stat(indexPath); err == nil && !indexInfo.IsDir() {
				fullPath = indexPath
				info = indexInfo
			} else if config.Browse {
				// Directory browsing not implemented in this optimization
				ctx.SendError("err_not_implemented", New(ErrInvalidRequest, "Directory browsing not implemented"))
				return
			} else {
				ctx.Send404()
				return
			}
		}

		// Handle caching headers
		etag := generateETag(info)
		ctx.SetHeader(HeaderETag, etag)
		
		if config.MaxAge > 0 {
			ctx.SetHeader(HeaderCacheControl, "public, max-age="+strconv.Itoa(config.MaxAge))
		}

		// Check if-none-match
		if match := ctx.GetHeader(HeaderIfNoneMatch); match == etag {
			ctx.SetStatus(http.StatusNotModified)
			ctx.Done()
			return
		}

		// Check if-modified-since
		if modifiedSince := ctx.GetHeader(HeaderIfModifiedSince); modifiedSince != "" {
			t, err := time.Parse(http.TimeFormat, modifiedSince)
			if err == nil && info.ModTime().Before(t.Add(1*time.Second)) {
				ctx.SetStatus(http.StatusNotModified)
				ctx.Done()
				return
			}
		}

		// Set content type
		contentType := getContentType(fullPath)
		ctx.SetHeader(HeaderContentType, contentType)
		ctx.SetHeader(HeaderLastModified, info.ModTime().UTC().Format(http.TimeFormat))


	// Skip body write for HEAD requests (avoids Go HTTP/2 bug #66812)
	if ctx.Request.Method == http.MethodHead {
		ctx.Done()
		return
	}

		// Serve from cache if enabled and file is small enough
		if config.EnableCaching && info.Size() < 10*1024*1024 { // Cache files under 10MB
			if cached := fileCache.get(fullPath); cached != nil {
				if cached.modTime.Equal(info.ModTime()) {
					ctx.ResponseWriter.Write(cached.content)
					ctx.Done()
					return
				}
			}

			// Read and cache file
			content, err := os.ReadFile(fullPath)
			if err != nil {
				ctx.SendError("err_internal_error", Wrap(err, ErrInternal, "Failed to read file"))
				return
			}

			// Cache the file
			fileCache.put(fullPath, &cachedFile{
				content:  content,
				modTime:  info.ModTime(),
				size:     info.Size(),
				etag:     etag,
				lastUsed: time.Now(),
			})

			ctx.ResponseWriter.Write(content)
			ctx.Done()
			return
		}

		// For large files, use zero-copy sendfile if available
		file, err := os.Open(fullPath)
		if err != nil {
			ctx.SendError("err_internal_error", Wrap(err, ErrInternal, "Failed to open file"))
			return
		}
		defer file.Close()

		// Use io.Copy which can use sendfile syscall on Linux
		_, copyErr := io.Copy(ctx.ResponseWriter, file)
		if copyErr != nil {
			// Log error but response may be partially sent
			if !EnableLoggerCheck || logger != nil {
				logger.Error().
					Err(copyErr).
					Str("path", fullPath).
					Str("ip", ctx.ClientIP()).
					Msg("[octo] failed to send file")
			}
		}
		ctx.Done()
	}
}

// generateETag generates an ETag for a file
func generateETag(info os.FileInfo) string {
	return `"` + info.ModTime().Format(time.RFC3339Nano) + `-` + strconv.FormatInt(info.Size(), 10) + `"`
}

// getContentType returns the content type for a file
func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".pdf":
		return "application/pdf"
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

// Cache methods
func (c *staticCache) get(path string) *cachedFile {
	if val, ok := c.files.Load(path); ok {
		file := val.(*cachedFile)
		// Update lastUsed atomically
		file.lastUsed = time.Now()
		return file
	}
	return nil
}

func (c *staticCache) put(path string, file *cachedFile) {
	// Try to add without locking first
	newSize := c.size.Add(file.size)
	
	// Check if we need eviction
	if newSize > c.maxSize {
		c.evictMu.Lock()
		c.evict()
		c.evictMu.Unlock()
	}
	
	c.files.Store(path, file)
}

func (c *staticCache) evict() {
	// Simple LRU eviction - remove oldest accessed file
	// Under high load, just remove 10% of cache entries to avoid long iterations
	type cacheEntry struct {
		path string
		file *cachedFile
	}
	
	var entries []cacheEntry
	count := 0
	
	// Collect entries (stop after reasonable amount to avoid blocking)
	c.files.Range(func(key, value interface{}) bool {
		count++
		if count > 100 { // Limit scanning to first 100 entries
			return false
		}
		entries = append(entries, cacheEntry{
			path: key.(string),
			file: value.(*cachedFile),
		})
		return true
	})
	
	if len(entries) == 0 {
		return
	}
	
	// Remove 10% of scanned entries (at least 1)
	toRemove := len(entries) / 10
	if toRemove < 1 {
		toRemove = 1
	}
	
	// Sort by last used time (oldest first)
	for i := 0; i < toRemove && i < len(entries); i++ {
		oldestIdx := i
		for j := i + 1; j < len(entries); j++ {
			if entries[j].file.lastUsed.Before(entries[oldestIdx].file.lastUsed) {
				oldestIdx = j
			}
		}
		// Swap
		if oldestIdx != i {
			entries[i], entries[oldestIdx] = entries[oldestIdx], entries[i]
		}
		// Remove the entry
		c.files.Delete(entries[i].path)
		c.size.Add(-entries[i].file.size)
	}
}