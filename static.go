package octo

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	mu       sync.RWMutex
	files    map[string]*cachedFile
	size     int64
	maxSize  int64
	maxFiles int
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
		files:    make(map[string]*cachedFile),
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
		ctx.SetHeader("ETag", etag)
		
		if config.MaxAge > 0 {
			ctx.SetHeader("Cache-Control", "public, max-age="+strconv.Itoa(config.MaxAge))
		}

		// Check if-none-match
		if match := ctx.GetHeader("If-None-Match"); match == etag {
			ctx.SetStatus(http.StatusNotModified)
			return
		}

		// Check if-modified-since
		if modifiedSince := ctx.GetHeader("If-Modified-Since"); modifiedSince != "" {
			t, err := time.Parse(http.TimeFormat, modifiedSince)
			if err == nil && info.ModTime().Before(t.Add(1*time.Second)) {
				ctx.SetStatus(http.StatusNotModified)
				return
			}
		}

		// Set content type
		contentType := getContentType(fullPath)
		ctx.SetHeader("Content-Type", contentType)
		ctx.SetHeader("Last-Modified", info.ModTime().UTC().Format(http.TimeFormat))

		// Serve from cache if enabled and file is small enough
		if config.EnableCaching && info.Size() < 10*1024*1024 { // Cache files under 10MB
			if cached := fileCache.get(fullPath); cached != nil {
				if cached.modTime.Equal(info.ModTime()) {
					ctx.ResponseWriter.Write(cached.content)
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
		io.Copy(ctx.ResponseWriter, file)
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
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if file, ok := c.files[path]; ok {
		file.lastUsed = time.Now()
		return file
	}
	return nil
}

func (c *staticCache) put(path string, file *cachedFile) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if we need to evict files
	if len(c.files) >= c.maxFiles || c.size+file.size > c.maxSize {
		c.evict()
	}
	
	c.files[path] = file
	c.size += file.size
}

func (c *staticCache) evict() {
	// Simple LRU eviction - remove oldest accessed file
	var oldestPath string
	var oldestTime time.Time
	
	for path, file := range c.files {
		if oldestPath == "" || file.lastUsed.Before(oldestTime) {
			oldestPath = path
			oldestTime = file.lastUsed
		}
	}
	
	if oldestPath != "" {
		c.size -= c.files[oldestPath].size
		delete(c.files, oldestPath)
	}
}