package octo

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

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
		root: &node[V]{},
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

func splitPath(path string) []string {
	if path == "" || path == "/" {
		return nil
	}
	if path[0] == '/' {
		path = path[1:]
	}
	segmentCount := 1
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			segmentCount++
		}
	}
	parts := make([]string, 0, segmentCount)
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
	path := req.URL.Path
	method := req.Method

	// 3) Optionally add security headers
	if EnableSecurityHeaders {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
	}

	handler, middlewareChain, params, ok := r.search(method, path)
	if !ok {
		handler = func(ctx *Ctx[V]) {
			if req.Method == "OPTIONS" {
				w.Header().Set("Allow", "GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD")
				w.WriteHeader(http.StatusOK)
				return
			}
			http.NotFound(ctx.ResponseWriter, ctx.Request)
		}
		middlewareChain = r.globalMiddlewareChain()
	}

	responseWriter := NewResponseWriterWrapper(w)

	ctx := &Ctx[V]{
		ResponseWriter: responseWriter,
		Request:        req,
		Params:         params,
		StartTime:      time.Now().UnixNano(),
		UUID:           uuid.NewString(),
		Query:          req.URL.Query(),
	}

	handler = applyMiddleware(handler, middlewareChain)
	handler(ctx)
}

func (r *Router[V]) search(method, path string) (HandlerFunc[V], []MiddlewareFunc[V], map[string]string, bool) {
	parts := splitPath(path)
	cur := r.root
	var paramValues []string

	for i, part := range parts {
		if part == "" {
			continue
		}
		if child, ok := cur.staticChildren[part]; ok {
			cur = child
			continue
		}

		// embedded param or standard param
		matched := false
		if cur.staticChildren != nil {
			for key, child := range cur.staticChildren {
				if strings.HasPrefix(part, key) {
					remaining := part[len(key):]
					if remaining != "" {
						cur = child
						part = remaining
						for {
							if cur.paramChild != nil {
								paramValues = append(paramValues, part)
								cur = cur.paramChild
								matched = true
								break
							}
							if cur.staticChildren != nil {
								found := false
								for k, c := range cur.staticChildren {
									if strings.HasPrefix(part, k) {
										cur = c
										part = part[len(k):]
										found = true
										break
									}
								}
								if !found {
									break
								}
							} else {
								break
							}
						}
						if matched {
							break
						}
					}
				}
			}
		}
		if matched {
			continue
		}

		if cur.paramChild != nil {
			paramValues = append(paramValues, part)
			cur = cur.paramChild
			continue
		}
		if cur.wildcardChild != nil {
			remainingParts := strings.Join(parts[i:], "/")
			paramValues = append(paramValues, remainingParts)
			cur = cur.wildcardChild
			break
		}
		return nil, nil, nil, false
	}

	handlerEntry, ok := cur.handlers[method]
	if !ok || !cur.isLeaf {
		return nil, nil, nil, false
	}
	var params map[string]string
	if len(handlerEntry.paramNames) > 0 {
		params = make(map[string]string, len(handlerEntry.paramNames))
		for i, paramName := range handlerEntry.paramNames {
			if i < len(paramValues) {
				params[paramName] = paramValues[i]
			}
		}
	}
	return handlerEntry.handler, handlerEntry.middleware, params, true
}

func wrapMiddleware[V any](mw MiddlewareFunc[V]) MiddlewareFunc[V] {
	return func(next HandlerFunc[V]) HandlerFunc[V] {
		return func(ctx *Ctx[V]) {
			if ctx.done {
				return
			}
			mw(next)(ctx)
		}
	}
}

func wrapHandler[V any](handler HandlerFunc[V]) HandlerFunc[V] {
	return func(ctx *Ctx[V]) {
		if ctx.done {
			return
		}
		handler(ctx)
	}
}

func applyMiddleware[V any](handler HandlerFunc[V], middleware []MiddlewareFunc[V]) HandlerFunc[V] {
	handler = wrapHandler(handler)
	for i := len(middleware) - 1; i >= 0; i-- {
		mw := wrapMiddleware(middleware[i])
		handler = mw(handler)
	}
	return handler
}

func RecoveryMiddleware[V any]() MiddlewareFunc[V] {
	return func(next HandlerFunc[V]) HandlerFunc[V] {
		return func(ctx *Ctx[V]) {
			defer func() {
				if err := recover(); err != nil {
					var pcs [32]uintptr
					n := runtime.Callers(3, pcs[:])
					frames := runtime.CallersFrames(pcs[:n])

					var stackLines []string
					for {
						frame, more := frames.Next()
						stackLines = append(stackLines, fmt.Sprintf("%s\n\t%s:%d", frame.Function, frame.File, frame.Line))
						if !more {
							break
						}
					}
					zStack := zerolog.Arr()
					for _, line := range stackLines {
						zStack.Str(line)
					}
					var wrappedErr error
					switch e := err.(type) {
					case error:
						wrappedErr = errors.WithStack(e)
					default:
						wrappedErr = errors.Errorf("%v", e)
					}
					if errors.Is(wrappedErr, http.ErrAbortHandler) {
						if EnableLoggerCheck {
							if logger != nil {
								logger.Warn().
									Str("path", ctx.Request.URL.Path).
									Str("method", ctx.Request.Method).
									Msg("[octo-panic] Client aborted request (panic recovered)")
							}
						} else {
							logger.Warn().
								Str("path", ctx.Request.URL.Path).
								Str("method", ctx.Request.Method).
								Msg("[octo-panic] Client aborted request (panic recovered)")
						}
						return
					}
					if EnableLoggerCheck {
						if logger != nil {
							logger.Error().
								Err(wrappedErr).
								Stack().
								Array("stack_array", zStack).
								Str("path", ctx.Request.URL.Path).
								Str("method", ctx.Request.Method).
								Str("ip", ctx.ClientIP()).
								Msg("[octo-panic] Panic recovered")
						}
					} else {
						logger.Error().
							Err(wrappedErr).
							Stack().
							Array("stack_array", zStack).
							Str("path", ctx.Request.URL.Path).
							Str("method", ctx.Request.Method).
							Str("ip", ctx.ClientIP()).
							Msg("[octo-panic] Panic recovered")
					}
					if !strings.Contains(ctx.ResponseWriter.Header().Get("Content-Type"), "application/json") {
						http.Error(ctx.ResponseWriter, "Internal Server Error", http.StatusInternalServerError)
					}
				}
			}()
			next(ctx)
		}
	}
}
