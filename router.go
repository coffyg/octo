package octo

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

type HandlerFunc[V any] func(*Ctx[V])
type MiddlewareFunc[V any] func(HandlerFunc[V]) HandlerFunc[V]
type routeEntry[V any] struct {
	handler    HandlerFunc[V]
	paramNames []string
}
type node[V any] struct {
	staticChildren map[string]*node[V]
	paramChild     *node[V]
	wildcardChild  *node[V]
	isLeaf         bool
	handlers       map[string]*routeEntry[V]
	middleware     []MiddlewareFunc[V]
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
	for _, method := range methods {
		r.addRoute(method, path, handler, middleware...)
	}
}

// Group represents a group of routes with a common prefix and middleware
type Group[V any] struct {
	prefix     string
	router     *Router[V]
	middleware []MiddlewareFunc[V]
}

// Group creates a new route group with the given prefix and middleware
func (r *Router[V]) Group(prefix string, middleware ...MiddlewareFunc[V]) *Group[V] {
	current := r.root
	parts := splitPath(prefix)
	for _, part := range parts {
		if part == "" {
			continue
		}
		if part[0] == ':' {
			if current.paramChild == nil {
				current.paramChild = &node[V]{}
			}
			current = current.paramChild
		} else {
			if current.staticChildren == nil {
				current.staticChildren = make(map[string]*node[V])
			}
			if current.staticChildren[part] == nil {
				current.staticChildren[part] = &node[V]{}
			}
			current = current.staticChildren[part]
		}
		// Assign middleware to current node, avoiding duplicates
		if len(middleware) > 0 {
			current.middleware = appendUniqueMiddleware(current.middleware, middleware...)
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
	g.router.GET(fullPath, handler, middleware...)
}

func (g *Group[V]) POST(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	g.router.POST(fullPath, handler, middleware...)
}

func (g *Group[V]) PUT(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	g.router.PUT(fullPath, handler, middleware...)
}

func (g *Group[V]) DELETE(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	g.router.DELETE(fullPath, handler, middleware...)
}

func (g *Group[V]) PATCH(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	g.router.PATCH(fullPath, handler, middleware...)
}

func (g *Group[V]) OPTIONS(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	g.router.OPTIONS(fullPath, handler, middleware...)
}

func (g *Group[V]) HEAD(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	g.router.HEAD(fullPath, handler, middleware...)
}

// ANY adds a route that matches all HTTP methods
func (g *Group[V]) ANY(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	fullPath := g.prefix + path
	g.router.ANY(fullPath, handler, middleware...)
}

// addRoute adds a route with associated handler and middleware
func (r *Router[V]) addRoute(method, path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	parts := splitPath(path)
	current := r.root

	var paramNames []string

	for i, part := range parts {
		if part == "" {
			continue
		}
		if part[0] == ':' {
			paramName := part[1:]
			paramNames = append(paramNames, paramName)
			if current.paramChild == nil {
				current.paramChild = &node[V]{}
			}
			current = current.paramChild
		} else if part[0] == '*' {
			paramName := part[1:]
			paramNames = append(paramNames, paramName)
			if current.wildcardChild == nil {
				current.wildcardChild = &node[V]{}
			}
			current = current.wildcardChild
			// Wildcard must be at the end
			if i != len(parts)-1 {
				panic("Wildcard route parameter must be at the end of the path")
			}
			break
		} else {
			if current.staticChildren == nil {
				current.staticChildren = make(map[string]*node[V])
			}
			if current.staticChildren[part] == nil {
				current.staticChildren[part] = &node[V]{}
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
	current.handlers[method] = &routeEntry[V]{handler: handler, paramNames: paramNames}

	if len(middleware) > 0 {
		current.middleware = appendUniqueMiddleware(current.middleware, middleware...)
	}
}

// splitPath splits the path into segments
func splitPath(path string) []string {
	if path == "" || path == "/" {
		return []string{}
	}
	if path[0] == '/' {
		path = path[1:]
	}
	return strings.Split(path, "/")
}

// search finds the handler and middleware chain for a given request
func (r *Router[V]) search(method, path string) (HandlerFunc[V], []MiddlewareFunc[V], map[string]string, bool) {
	parts := splitPath(path)
	current := r.root

	var middlewareChain []MiddlewareFunc[V]
	var paramsValues []string

	for i, part := range parts {
		if len(current.middleware) > 0 {
			middlewareChain = append(middlewareChain, current.middleware...)
		}

		if child, ok := current.staticChildren[part]; ok {
			current = child
			continue
		}

		if current.paramChild != nil {
			paramsValues = append(paramsValues, part)
			current = current.paramChild
			continue
		}

		if current.wildcardChild != nil {
			// Remaining parts are matched to wildcard parameter
			remainingParts := strings.Join(parts[i:], "/")
			paramsValues = append(paramsValues, remainingParts)
			current = current.wildcardChild
			if len(current.middleware) > 0 {
				middlewareChain = append(middlewareChain, current.middleware...)
			}
			break
		}

		// No matching child
		return nil, nil, nil, false
	}

	if len(current.middleware) > 0 {
		middlewareChain = append(middlewareChain, current.middleware...)
	}

	handlerEntry, ok := current.handlers[method]
	if !ok || !current.isLeaf {
		return nil, nil, nil, false
	}

	// Build the params map
	params := make(map[string]string)
	for i, paramName := range handlerEntry.paramNames {
		if i < len(paramsValues) {
			params[paramName] = paramsValues[i]
		}
	}

	return handlerEntry.handler, middlewareChain, params, true
}

// applyMiddleware wraps the handler with middleware functions
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
	// Wrap the handler
	handler = wrapHandler(handler)
	// Wrap and apply middleware
	for i := len(middleware) - 1; i >= 0; i-- {
		mw := wrapMiddleware(middleware[i])
		handler = mw(handler)
	}
	return handler
}

// ServeHTTP implements the http.Handler interface
func (r *Router[V]) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	method := req.Method

	handler, middlewareChain, params, ok := r.search(method, path)

	// Combine middleware in the correct order
	allMiddleware := append(r.preGroupMiddleware, r.middleware...)
	allMiddleware = append(allMiddleware, middlewareChain...)

	if !ok {
		// Route not found, define a default handler
		handler = func(ctx *Ctx[V]) {
			if req.Method == "OPTIONS" {
				w.Header().Set("Allow", "GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD")
				w.WriteHeader(http.StatusOK)
				return
			}
			http.NotFound(ctx.ResponseWriter, ctx.Request)
		}
	}

	ctx := &Ctx[V]{
		ResponseWriter: w,
		Request:        req,
		Params:         params,
		StartTime:      time.Now().UnixNano(),
		UUID:           uuid.New().String(),
		Query:          req.URL.Query(),
	}

	handler = applyMiddleware(handler, allMiddleware)

	handler(ctx)
}

// appendUniqueMiddleware appends middleware functions to a slice, avoiding duplicates
func appendUniqueMiddleware[V any](existing []MiddlewareFunc[V], newMiddleware ...MiddlewareFunc[V]) []MiddlewareFunc[V] {
	existingSet := make(map[uintptr]struct{}, len(existing))
	for _, emw := range existing {
		existingSet[reflect.ValueOf(emw).Pointer()] = struct{}{}
	}
	for _, nmw := range newMiddleware {
		ptr := reflect.ValueOf(nmw).Pointer()
		if _, found := existingSet[ptr]; !found {
			existing = append(existing, nmw)
			existingSet[ptr] = struct{}{}
		}
	}
	return existing
}
