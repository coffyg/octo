package octo

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type HandlerFunc[V any] func(*Ctx[V])
type MiddlewareFunc[V any] func(HandlerFunc[V]) HandlerFunc[V]

type node[V any] struct {
	staticChildren map[string]*node[V] // Static path segments
	paramChild     *node[V]            // Parameterized path segment
	paramName      string              // Name of the path parameter
	isLeaf         bool                // Indicates if the node is a leaf
	handlers       map[string]HandlerFunc[V]
	middleware     []MiddlewareFunc[V]
}

type Router[V any] struct {
	root       *node[V]
	middleware []MiddlewareFunc[V]
}

var hasBody = map[string]bool{
	"POST":   true,
	"PUT":    true,
	"PATCH":  true,
	"DELETE": true,
}

func NewRouter[V any]() *Router[V] {
	return &Router[V]{
		root: &node[V]{},
	}
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
	return &Group[V]{
		prefix:     prefix,
		router:     r,
		middleware: middleware,
	}
}

// Methods to add routes to the group
func (g *Group[V]) GET(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	combinedMiddleware := append(g.middleware, middleware...)
	fullPath := g.prefix + path
	g.router.GET(fullPath, handler, combinedMiddleware...)
}

func (g *Group[V]) POST(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	combinedMiddleware := append(g.middleware, middleware...)
	fullPath := g.prefix + path
	g.router.POST(fullPath, handler, combinedMiddleware...)
}

func (g *Group[V]) PUT(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	combinedMiddleware := append(g.middleware, middleware...)
	fullPath := g.prefix + path
	g.router.PUT(fullPath, handler, combinedMiddleware...)
}

func (g *Group[V]) DELETE(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	combinedMiddleware := append(g.middleware, middleware...)
	fullPath := g.prefix + path
	g.router.DELETE(fullPath, handler, combinedMiddleware...)
}

func (g *Group[V]) PATCH(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	combinedMiddleware := append(g.middleware, middleware...)
	fullPath := g.prefix + path
	g.router.PATCH(fullPath, handler, combinedMiddleware...)
}

func (g *Group[V]) OPTIONS(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	combinedMiddleware := append(g.middleware, middleware...)
	fullPath := g.prefix + path
	g.router.OPTIONS(fullPath, handler, combinedMiddleware...)
}

func (g *Group[V]) HEAD(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	combinedMiddleware := append(g.middleware, middleware...)
	fullPath := g.prefix + path
	g.router.HEAD(fullPath, handler, combinedMiddleware...)
}

// ANY adds a route that matches all HTTP methods
func (g *Group[V]) ANY(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	combinedMiddleware := append(g.middleware, middleware...)
	fullPath := g.prefix + path
	g.router.ANY(fullPath, handler, combinedMiddleware...)
}

// addRoute adds a route with associated handler and middleware
func (r *Router[V]) addRoute(method, path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
	parts := splitPath(path)
	current := r.root

	for _, part := range parts {
		if part == "" {
			continue
		}
		if part[0] == ':' {
			paramName := part[1:]
			if current.paramChild == nil {
				current.paramChild = &node[V]{}
				current.paramChild.paramName = paramName
			} else if current.paramChild.paramName != paramName {
				panic(fmt.Sprintf("conflicting parameter name: %s", part))
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
		if len(middleware) > 0 {
			if current.middleware == nil {
				current.middleware = make([]MiddlewareFunc[V], 0)
			}
			current.middleware = append(current.middleware, middleware...)
		}
	}

	if current.handlers == nil {
		current.handlers = make(map[string]HandlerFunc[V])
	}

	if _, exists := current.handlers[method]; exists {
		panic(fmt.Sprintf("route already defined: %s %s", method, path))
	}

	current.isLeaf = true
	current.handlers[method] = handler
	if len(middleware) > 0 {
		current.middleware = middleware
	}
}

// splitPath splits the path into segments
/*
func splitPath(path string) []string {
	if path == "/" {
		return []string{}
	}
	parts := strings.Split(path, "/")
	if parts[0] == "" {
		parts = parts[1:]
	}
	return parts
}
*/
func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if start < i {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

// search finds the handler and middleware chain for a given request
func (r *Router[V]) search(method, path string, params map[string]string) (HandlerFunc[V], []MiddlewareFunc[V], bool) {
	parts := splitPath(path)
	current := r.root

	var middlewareChain []MiddlewareFunc[V]

	for _, part := range parts {
		if current.staticChildren != nil {
			if child, ok := current.staticChildren[part]; ok {
				current = child
				if len(current.middleware) > 0 {
					middlewareChain = append(middlewareChain, current.middleware...)
				}
				continue
			}
		}
		if current.paramChild != nil {
			current = current.paramChild
			params[current.paramName] = part
			if len(current.middleware) > 0 {
				middlewareChain = append(middlewareChain, current.middleware...)
			}
		} else {
			return nil, nil, false
		}
	}

	handler, ok := current.handlers[method]
	return handler, middlewareChain, ok && current.isLeaf
}

// applyMiddleware wraps the handler with middleware functions
func applyMiddleware[V any](handler HandlerFunc[V], middleware []MiddlewareFunc[V]) HandlerFunc[V] {
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

// ServeHTTP implements the http.Handler interface
func (r *Router[V]) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	method := req.Method

	params := make(map[string]string)
	handler, middlewareChain, ok := r.search(method, path, params)

	if !ok {
		http.NotFound(w, req)
		return
	}

	if req.Method == "OPTIONS" {
		w.Header().Set("Allow", "GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD")
		w.WriteHeader(http.StatusOK)
		return
	}

	var body []byte
	if hasBody[req.Method] {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
	}

	ctx := &Ctx[V]{
		ResponseWriter: w,
		Request:        req,
		Params:         params,
		StartTime:      time.Now().UnixNano(),
		UUID:           uuid.New().String(),
		Body:           body,
		Query:          make(map[string]*[]string),
	}

	query := req.URL.Query()
	for key, value := range query {
		ctx.Query[key] = &value
	}

	// Combine global and route-specific middleware
	allMiddleware := append(r.middleware, middlewareChain...)
	handler = applyMiddleware(handler, allMiddleware)

	handler(ctx)
}
