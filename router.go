package octo

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// getConnectionTypeName returns a string representation of the connection type
func getConnectionTypeName(ct ConnectionType) string {
	switch ct {
	case ConnectionTypeSSE:
		return "SSE"
	case ConnectionTypeWebSocket:
		return "WebSocket"
	default:
		return "HTTP"
	}
}

type HandlerFunc[V any] func(*Ctx[V])
type MiddlewareFunc[V any] func(HandlerFunc[V]) HandlerFunc[V]

type routeEntry[V any] struct {
	handler    HandlerFunc[V]
	paramNames []string
	middleware []MiddlewareFunc[V]
}

type node[V any] struct {
	staticChildren map[string]*node[V]
	paramChild     *node[V]
	wildcardChild  *node[V]
	isLeaf         bool
	handlers       map[string]*routeEntry[V]
	middleware     []MiddlewareFunc[V]
	parent         *node[V]
}

type Router[V any] struct {
	root               *node[V]
	middleware         []MiddlewareFunc[V]
	preGroupMiddleware []MiddlewareFunc[V]
}

func NewRouter[V any]() *Router[V] {

	return &Router[V]{
		root: &node[V]{
			staticChildren: make(map[string]*node[V], 8), // Pre-allocate common size
		},
	}

}

// UseGlobal adds middleware that applies to all routes before group middleware
func (r *Router[V]) UseGlobal(mw MiddlewareFunc[V]) {
	r.preGroupMiddleware = append(r.preGroupMiddleware, mw)
}

// Use adds a global middleware to the router
func (r *Router[V]) Use(mw MiddlewareFunc[V]) {
	r.middleware = append(r.middleware, mw)
}

// HTTP method handlers with optional route-specific middleware
func (r *Router[V]) GET(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	r.addRoute("GET", path, handler, middleware...)
}

func (r *Router[V]) POST(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	r.addRoute("POST", path, handler, middleware...)
}

func (r *Router[V]) PUT(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	r.addRoute("PUT", path, handler, middleware...)
}

func (r *Router[V]) DELETE(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	r.addRoute("DELETE", path, handler, middleware...)
}

func (r *Router[V]) PATCH(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	r.addRoute("PATCH", path, handler, middleware...)
}

func (r *Router[V]) OPTIONS(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	r.addRoute("OPTIONS", path, handler, middleware...)
}

func (r *Router[V]) HEAD(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	r.addRoute("HEAD", path, handler, middleware...)
}

func (r *Router[V]) ANY(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	for _, m := range methods {
		r.addRoute(m, path, handler, middleware...)
	}
}

// Group represents a group of routes with a common prefix and middleware
type Group[V any] struct {
	prefix     string
	router     *Router[V]
	middleware []MiddlewareFunc[V]
}

func (g *Group[V]) Use(mw MiddlewareFunc[V]) {
	g.middleware = append(g.middleware, mw)
}

// Group creates a new route group with the given prefix and middleware
func (r *Router[V]) Group(prefix string, middleware ...MiddlewareFunc[V]) *Group[V] {
	current := r.root
	parts := splitPath(prefix)
	for _, part := range parts {
		if part == "" {
			continue
		}
		if part[0] == ':' || strings.Contains(part, ":") {
			current = r.addEmbeddedParameterNode(current, part)
		} else {
			if current.staticChildren == nil {
				current.staticChildren = make(map[string]*node[V])
			}
			if current.staticChildren[part] == nil {
				current.staticChildren[part] = &node[V]{parent: current}
			}
			current = current.staticChildren[part]
		}
	}
	return &Group[V]{
		prefix:     prefix,
		router:     r,
		middleware: middleware,
	}
}

// Methods to add routes to the group
func (g *Group[V]) GET(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	allMiddleware := append(g.middleware, middleware...)
	g.router.GET(fullPath, handler, allMiddleware...)
}

func (g *Group[V]) POST(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	allMiddleware := append(g.middleware, middleware...)
	g.router.POST(fullPath, handler, allMiddleware...)
}

func (g *Group[V]) PUT(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	allMiddleware := append(g.middleware, middleware...)
	g.router.PUT(fullPath, handler, allMiddleware...)
}

func (g *Group[V]) DELETE(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	allMiddleware := append(g.middleware, middleware...)
	g.router.DELETE(fullPath, handler, allMiddleware...)
}

func (g *Group[V]) PATCH(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	allMiddleware := append(g.middleware, middleware...)
	g.router.PATCH(fullPath, handler, allMiddleware...)
}

func (g *Group[V]) OPTIONS(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	allMiddleware := append(g.middleware, middleware...)
	g.router.OPTIONS(fullPath, handler, allMiddleware...)
}

func (g *Group[V]) HEAD(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	allMiddleware := append(g.middleware, middleware...)
	g.router.HEAD(fullPath, handler, allMiddleware...)
}

// ANY adds a route that matches all HTTP methods
func (g *Group[V]) ANY(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	allMiddleware := append(g.middleware, middleware...)
	g.router.ANY(fullPath, handler, allMiddleware...)
}

// addRoute adds a route with associated handler and middleware
func (r *Router[V]) addRoute(method, path string, handler HandlerFunc[V], routeMW ...MiddlewareFunc[V]) {
	parts := splitPath(path)
	current := r.root

	var paramNames []string

	for i, part := range parts {
		if part == "" {
			continue
		}
		if strings.Contains(part, ":") {
			current, paramNames = r.addEmbeddedParameterNodeWithNames(current, part, paramNames)
		} else if part[0] == '*' {
			// Wildcard segment
			paramName := part[1:]
			paramNames = append(paramNames, paramName)
			if current.wildcardChild == nil {
				current.wildcardChild = &node[V]{parent: current}
			}
			current = current.wildcardChild
			if i != len(parts)-1 {
				panic("Wildcard route parameter must be at the end of the path")
			}
		} else {
			if current.staticChildren == nil {
				current.staticChildren = make(map[string]*node[V])
			}
			if current.staticChildren[part] == nil {
				current.staticChildren[part] = &node[V]{parent: current}
			}
			current = current.staticChildren[part]
		}
	}

	if current.handlers == nil {
		current.handlers = make(map[string]*routeEntry[V])
	}

	if _, exists := current.handlers[method]; exists {
		panic(fmt.Sprintf("route already defined: %s %s", method, path))
	}

	current.isLeaf = true

	// Build the middleware chain
	middlewareChain := r.buildMiddlewareChain(current, routeMW)
	current.handlers[method] = &routeEntry[V]{
		handler:    handler,
		paramNames: paramNames,
		middleware: middlewareChain,
	}
}

func (r *Router[V]) addEmbeddedParameterNodeWithNames(cur *node[V], part string, paramNames []string) (*node[V], []string) {
	for {
		if part == "" {
			break
		}
		idx := strings.IndexByte(part, ':')
		if idx == -1 {
			// Remaining part is static
			if cur.staticChildren == nil {
				cur.staticChildren = make(map[string]*node[V])
			}
			if cur.staticChildren[part] == nil {
				cur.staticChildren[part] = &node[V]{parent: cur}
			}
			cur = cur.staticChildren[part]
			break
		}
		if idx > 0 {
			staticPart := part[:idx]
			if cur.staticChildren == nil {
				cur.staticChildren = make(map[string]*node[V])
			}
			if cur.staticChildren[staticPart] == nil {
				cur.staticChildren[staticPart] = &node[V]{parent: cur}
			}
			cur = cur.staticChildren[staticPart]
		}
		part = part[idx+1:]
		var paramName string
		nextIdx := strings.IndexAny(part, ":*")
		if nextIdx != -1 {
			paramName = part[:nextIdx]
			part = part[nextIdx:]
		} else {
			paramName = part
			part = ""
		}
		paramNames = append(paramNames, paramName)
		if cur.paramChild == nil {
			cur.paramChild = &node[V]{parent: cur}
		}
		cur = cur.paramChild
	}
	return cur, paramNames
}

func (r *Router[V]) addEmbeddedParameterNode(cur *node[V], part string) *node[V] {
	for {
		if part == "" {
			break
		}
		idx := strings.IndexByte(part, ':')
		if idx == -1 {
			if cur.staticChildren == nil {
				cur.staticChildren = make(map[string]*node[V])
			}
			if cur.staticChildren[part] == nil {
				cur.staticChildren[part] = &node[V]{parent: cur}
			}
			cur = cur.staticChildren[part]
			break
		}
		if idx > 0 {
			staticPart := part[:idx]
			if cur.staticChildren == nil {
				cur.staticChildren = make(map[string]*node[V])
			}
			if cur.staticChildren[staticPart] == nil {
				cur.staticChildren[staticPart] = &node[V]{parent: cur}
			}
			cur = cur.staticChildren[staticPart]
		}
		part = part[idx+1:]
		nextIdx := strings.IndexAny(part, ":*")
		if nextIdx != -1 {
			part = part[nextIdx:]
		} else {
			part = ""
		}
		if cur.paramChild == nil {
			cur.paramChild = &node[V]{parent: cur}
		}
		cur = cur.paramChild
	}
	return cur
}

func (r *Router[V]) buildMiddlewareChain(cur *node[V], routeMW []MiddlewareFunc[V]) []MiddlewareFunc[V] {
	var chain []MiddlewareFunc[V]
	chain = append(chain, r.preGroupMiddleware...)
	chain = append(chain, r.middleware...)

	// collect middleware from parent nodes
	var nodeMW []MiddlewareFunc[V]
	temp := cur
	for temp != nil {
		if len(temp.middleware) > 0 {
			nodeMW = append(nodeMW, temp.middleware...)
		}
		temp = temp.parent
	}
	// reverse them so parent-most is first
	for i := len(nodeMW) - 1; i >= 0; i-- {
		chain = append(chain, nodeMW[i])
	}
	chain = append(chain, routeMW...)
	return chain
}

func (r *Router[V]) globalMiddlewareChain() []MiddlewareFunc[V] {
	var chain []MiddlewareFunc[V]
	if len(r.preGroupMiddleware) > 0 {
		chain = append(chain, r.preGroupMiddleware...)
	}
	if len(r.middleware) > 0 {
		chain = append(chain, r.middleware...)
	}
	return chain
}

// pathSegment represents a segment of the path without allocation
type pathSegment struct {
	start int
	end   int
}

// splitPathZeroAlloc splits a path into segments without allocating strings
// Returns the segments as start/end indices in the original path
func splitPathZeroAlloc(path string) []pathSegment {
	if len(path) == 0 || path == "/" {
		return nil
	}

	// Remove leading slash
	start := 0
	if path[0] == '/' {
		start = 1
	}

	// Fast path for single segment
	if len(path) < 3 || !strings.Contains(path[start:], "/") {
		if start < len(path) {
			return []pathSegment{{start: start, end: len(path)}}
		}
		return nil
	}

	// Pre-count segments for exact allocation
	segmentCount := 1
	for i := start; i < len(path); i++ {
		if path[i] == '/' {
			segmentCount++
		}
	}

	// Pre-allocate slice with exact capacity
	segments := make([]pathSegment, 0, segmentCount)

	// Split the path using indices
	segStart := start
	for i := start; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if segStart < i {
				segments = append(segments, pathSegment{start: segStart, end: i})
			}
			segStart = i + 1
		}
	}
	return segments
}

func splitPath(path string) []string {
	if path == "" || path == "/" {
		return nil
	}

	// Remove leading slash
	if path[0] == '/' {
		path = path[1:]
	}

	// For short paths, avoid unnecessary allocations
	if len(path) < 3 && !strings.Contains(path, "/") {
		return []string{path}
	}

	// Pre-count segments for better slice allocation
	segmentCount := 1
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			segmentCount++
		}
	}

	// Pre-allocate slice with exact capacity
	parts := make([]string, 0, segmentCount)

	// Split the path
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if start < i {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	return parts
}

// ServeHTTP implements the http.Handler interface
func (r *Router[V]) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Get core request info
	path := req.URL.Path
	method := req.Method

	// Optionally add security headers - only add if enabled
	if EnableSecurityHeaders {
		header := w.Header()
		header.Set(HeaderXContentTypeOptions, "nosniff")
		header.Set(HeaderXFrameOptions, "DENY")
		header.Set(HeaderXXSSProtection, "1; mode=block")
	}

	// Find matching route
	handler, middlewareChain, params, ok := r.search(method, path)

	// Handle not found case
	if !ok {
		handler = func(ctx *Ctx[V]) {
			// Fast path for OPTIONS requests
			if req.Method == "OPTIONS" {
				w.Header().Set(HeaderAllow, "GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD")
				w.WriteHeader(http.StatusOK)
				// Nothing to write so no error to check
				return
			}
			// Use Send404 which includes path information in logs
			ctx.Send404()
		}
		middlewareChain = r.globalMiddlewareChain()
	}

	// Wrap the response writer
	responseWriter := NewResponseWriterWrapper(w)

	// Get or create context
	// Create new context
	ctx := &Ctx[V]{
		ResponseWriter: responseWriter,
		Request:        req,
		StartTime:      time.Now().UnixNano(),
		UUID:           uuid.NewString(),           // Fast request ID instead of UUID
		Query:          req.URL.Query(),            // Parse upfront to maintain backward compatibility
		Params:         make(map[string]string, 4), // Pre-allocate for common case
	}
	
	// Detect connection type early
	ctx.DetectConnectionType()
	
	// Disable write deadline for streaming connections (SSE/WebSocket)
	if ctx.IsStreamingConnection() {
		rc := http.NewResponseController(w)
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			if !EnableLoggerCheck || logger != nil {
				logger.Warn().
					Err(err).
					Str("path", req.URL.Path).
					Str("conn_type", getConnectionTypeName(ctx.ConnectionType)).
					Msg("[octo-router] Failed to disable write deadline for streaming connection")
			}
		} else {
			if !EnableLoggerCheck || logger != nil {
				logger.Debug().
					Str("path", req.URL.Path).
					Str("conn_type", getConnectionTypeName(ctx.ConnectionType)).
					Msg("[octo-router] Disabled write deadline for streaming connection")
			}
		}
	}

	// Handle parameters
	if params != nil {
		// Reuse existing map if possible
		if ctx.Params == nil {
			ctx.Params = params
		} else {
			// Clear and copy
			for k := range ctx.Params {
				delete(ctx.Params, k)
			}
			for k, v := range params {
				ctx.Params[k] = v
			}
		}
	} else {
		// Clear params if no new params
		for k := range ctx.Params {
			delete(ctx.Params, k)
		}
	}

	// Apply middleware chain and execute
	handler = applyMiddleware(handler, middlewareChain)
	
	// Ensure cleanup happens even if handler panics
	defer func() {
		if ctx.ResponseWriter != nil && ctx.ResponseWriter.Body != nil {
			ctx.ResponseWriter.Body.Reset()
			bufferPool.Put(ctx.ResponseWriter.Body)
		}
	}()
	
	handler(ctx)

}

// search performs zero-allocation path matching
func (r *Router[V]) search(method, path string) (HandlerFunc[V], []MiddlewareFunc[V], map[string]string, bool) {
	// Fast path for root or empty path
	if path == "" || path == "/" {
		if r.root.isLeaf {
			if handlerEntry, ok := r.root.handlers[method]; ok {
				return handlerEntry.handler, handlerEntry.middleware, nil, true
			}
		}
		return nil, nil, nil, false
	}

	// Fast path for common static routes (no parameters)
	// Check if path contains parameter markers for early exit
	if !strings.ContainsAny(path, ":*") {
		// Try direct static lookup first
		if handler, middleware, ok := r.tryStaticFastPath(method, path); ok {
			return handler, middleware, nil, true
		}
	}

	// Get path segments without allocation
	segments := splitPathZeroAlloc(path)
	if len(segments) == 0 {
		// Handle paths like "///" by checking root handler
		if r.root.isLeaf {
			if handlerEntry, ok := r.root.handlers[method]; ok {
				return handlerEntry.handler, handlerEntry.middleware, nil, true
			}
		}
		return nil, nil, nil, false
	}
	
	// Prevent DoS via extremely long paths
	if len(segments) > 100 {
		return nil, nil, nil, false
	}

	cur := r.root
	// Pre-allocate parameter values slice to avoid multiple allocations
	paramValues := make([]pathSegment, 0, 8) // Most paths have fewer than 8 parameters

	for _, segment := range segments {
		if segment.start >= segment.end {
			continue
		}

		// Try static child first (most common case)
		if cur.staticChildren != nil {
			// Extract segment string only when needed
			segStr := path[segment.start:segment.end]
			if child, ok := cur.staticChildren[segStr]; ok {
				cur = child
				continue
			}

			// Try embedded parameter matching
			matched := false
			for key, child := range cur.staticChildren {
				if len(key) <= len(segStr) && segStr[:len(key)] == key {
					remaining := segStr[len(key):]
					if remaining != "" {
						cur = child
						// Create a new segment for the remaining part
						remainingSegment := pathSegment{
							start: segment.start + len(key),
							end:   segment.end,
						}

						// Handle nested parameter patterns
						for {
							if cur.paramChild != nil {
								paramValues = append(paramValues, remainingSegment)
								cur = cur.paramChild
								matched = true
								break
							}

							if cur.staticChildren == nil {
								break
							}

							// Check for static prefix matches
							found := false
							remainingStr := path[remainingSegment.start:remainingSegment.end]
							for k, c := range cur.staticChildren {
								if len(k) <= len(remainingStr) && remainingStr[:len(k)] == k {
									cur = c
									remainingSegment.start += len(k)
									found = true
									break
								}
							}

							if !found {
								break
							}
						}

						if matched {
							break
						}
					}
				}
			}

			if matched {
				continue
			}
		}

		// Try parameter child
		if cur.paramChild != nil {
			paramValues = append(paramValues, segment)
			cur = cur.paramChild
			continue
		}

		// Try wildcard child (lowest priority)
		if cur.wildcardChild != nil {
			// Create segment for remaining path
			wildcardSegment := pathSegment{
				start: segment.start,
				end:   len(path),
			}
			paramValues = append(paramValues, wildcardSegment)
			cur = cur.wildcardChild
			break
		}

		// No match found for this segment
		return nil, nil, nil, false
	}

	// Check if we reached a leaf node with a handler for this method
	if !cur.isLeaf {
		return nil, nil, nil, false
	}

	handlerEntry, ok := cur.handlers[method]
	if !ok {
		return nil, nil, nil, false
	}

	// Create parameter map only when needed
	var params map[string]string
	if len(handlerEntry.paramNames) > 0 && len(paramValues) > 0 {
		params = make(map[string]string, len(handlerEntry.paramNames))
		for i, paramName := range handlerEntry.paramNames {
			if i < len(paramValues) {
				// Extract parameter value from path using segment indices
				segment := paramValues[i]
				params[paramName] = path[segment.start:segment.end]
			}
		}
	}

	return handlerEntry.handler, handlerEntry.middleware, params, true
}

// tryStaticFastPath attempts a direct lookup for static routes
func (r *Router[V]) tryStaticFastPath(method, path string) (HandlerFunc[V], []MiddlewareFunc[V], bool) {
	segments := splitPath(path)
	if len(segments) == 0 {
		return nil, nil, false
	}

	cur := r.root
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if cur.staticChildren == nil {
			return nil, nil, false
		}
		child, ok := cur.staticChildren[segment]
		if !ok {
			return nil, nil, false
		}
		cur = child
	}

	if !cur.isLeaf {
		return nil, nil, false
	}

	handlerEntry, ok := cur.handlers[method]
	if !ok {
		return nil, nil, false
	}

	return handlerEntry.handler, handlerEntry.middleware, true
}

// Optimized middleware application that preserves the original middleware execution order
// This approach ensures that middleware is applied in the correct sequence:
// 1. First middleware in the chain (middleware[0]) runs first
// 2. Last middleware in the chain (middleware[len-1]) runs last
// 3. The handler runs after all middleware
//
// Performance optimizations:
// - Processing middleware in reverse order for proper nesting
// - Single ctx.done check per function call
// - Avoids unnecessary nested function calls with simpler implementation
func applyMiddleware[V any](handler HandlerFunc[V], middleware []MiddlewareFunc[V]) HandlerFunc[V] {
	// Fast path for no middleware
	if len(middleware) == 0 {
		return func(ctx *Ctx[V]) {
			if ctx.done {
				return
			}
			handler(ctx)
		}
	}

	// Apply middleware in reverse order to get the correct execution sequence
	// Last middleware in the chain (middleware[len-1]) wraps the handler first
	// First middleware in the chain (middleware[0]) is the outermost wrapper
	result := handler
	for i := len(middleware) - 1; i >= 0; i-- {
		mw := middleware[i]
		prev := result
		result = func(ctx *Ctx[V]) {
			if ctx.done {
				return
			}
			mw(prev)(ctx)
		}
	}

	return result
}

func RecoveryMiddleware[V any]() MiddlewareFunc[V] {
	return func(next HandlerFunc[V]) HandlerFunc[V] {
		return func(ctx *Ctx[V]) {
			defer func() {
				if r := recover(); r != nil {
					// Verify context is valid for panic handling
					if ctx == nil {
						// Critical case - can't do much except log to stderr
						fmt.Fprintf(os.Stderr, "CRITICAL: Panic with nil context: %v\n", r)
						return
					}

					if ctx.Request == nil {
						// Log panic but with limited context available
						LogPanic(logger, r, debug.Stack())
						return
					}

					// Get error object from the recovered panic
					var wrappedErr error
					switch e := r.(type) {
					case error:
						wrappedErr = errors.WithStack(e)
					default:
						wrappedErr = errors.Errorf("%v", r)
					}

					// Handle client aborted requests differently (less severe)
					if errors.Is(wrappedErr, http.ErrAbortHandler) {
						// For streaming connections, client disconnects are expected
						// Check path directly here as well as a fallback
						isSSEPath := strings.Contains(ctx.Request.URL.Path, "/sse") || strings.Contains(ctx.Request.URL.Path, "/events")
						if ctx.IsStreamingConnection() || isSSEPath {
							// Skip logging entirely or use debug level
							if !EnableLoggerCheck || logger != nil {
								logger.Debug().
									Str("path", ctx.Request.URL.Path).
									Str("method", ctx.Request.Method).
									Str("ip", ctx.ClientIP()).
									Str("conn_type", getConnectionTypeName(ctx.ConnectionType)).
									Bool("is_streaming", ctx.IsStreamingConnection()).
									Bool("is_sse_path", isSSEPath).
									Msg("[octo-stream] Client disconnected from streaming connection")
							}
						} else {
							// For regular HTTP, log as warning
							if !EnableLoggerCheck || logger != nil {
								logger.Warn().
									Str("path", ctx.Request.URL.Path).
									Str("method", ctx.Request.Method).
									Str("ip", ctx.ClientIP()).
									Int("conn_type_int", int(ctx.ConnectionType)).
									Msg("[octo-panic] Client aborted request (panic recovered)")
							}
						}
						return
					}

					// For other panics, use our enhanced panic logging
					// Only log if logger is enabled and available
					if !EnableLoggerCheck || logger != nil {
						// Use the improved human-readable panic logging
						LogPanicWithRequestInfo(
							logger,
							r,
							debug.Stack(),
							ctx.Request.URL.Path,
							ctx.Request.Method,
							ctx.ClientIP())
					}

					// Return an Internal Server Error response
					// Check if we need to respond with JSON or plain text
					contentType := ctx.ResponseWriter.Header().Get("Content-Type")
					if ctx.ResponseWriter != nil && !strings.Contains(contentType, "application/json") {
						// Send plain error message for non-JSON requests
						http.Error(ctx.ResponseWriter, "Internal Server Error", http.StatusInternalServerError)
					}
				}
			}()
			next(ctx)
		}
	}
}
