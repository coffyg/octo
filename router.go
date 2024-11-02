package octo

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
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
		if part[0] == ':' || strings.Contains(part, ":") {
			// Handle embedded parameter in group prefix
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
		// Assign middleware to current node
		if len(middleware) > 0 {
			current.middleware = append(current.middleware, middleware...)
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

		if part[0] == ':' || strings.Contains(part, ":") {
			// Handle embedded parameter
			current, paramNames = r.addEmbeddedParameterNodeWithNames(current, part, paramNames)
		} else if part[0] == '*' {
			// Wildcard parameter
			paramName := part[1:]
			paramNames = append(paramNames, paramName)
			if current.wildcardChild == nil {
				current.wildcardChild = &node[V]{parent: current}
			}
			current = current.wildcardChild
			// Wildcard must be at the end
			if i != len(parts)-1 {
				panic("Wildcard route parameter must be at the end of the path")
			}
			break
		} else {
			// Static segment
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
	middlewareChain := r.buildMiddlewareChain(current, middleware)

	current.handlers[method] = &routeEntry[V]{handler: handler, paramNames: paramNames, middleware: middlewareChain}

	// Assign route-specific middleware to the node (optional)
	if len(middleware) > 0 {
		current.middleware = append(current.middleware, middleware...)
	}
}

// Helper function to handle embedded parameters during route addition
func (r *Router[V]) addEmbeddedParameterNodeWithNames(current *node[V], part string, paramNames []string) (*node[V], []string) {
	for {
		if part == "" {
			break
		}

		idx := strings.IndexByte(part, ':')
		if idx == -1 {
			// Remaining part is static
			if current.staticChildren == nil {
				current.staticChildren = make(map[string]*node[V])
			}
			if current.staticChildren[part] == nil {
				current.staticChildren[part] = &node[V]{parent: current}
			}
			current = current.staticChildren[part]
			break
		}

		// Static part before ':'
		if idx > 0 {
			staticPart := part[:idx]
			if current.staticChildren == nil {
				current.staticChildren = make(map[string]*node[V])
			}
			if current.staticChildren[staticPart] == nil {
				current.staticChildren[staticPart] = &node[V]{parent: current}
			}
			current = current.staticChildren[staticPart]
		}

		// Parameter part after ':'
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
		if current.paramChild == nil {
			current.paramChild = &node[V]{parent: current}
		}
		current = current.paramChild
	}
	return current, paramNames
}

// Helper function to handle embedded parameters in group prefixes
func (r *Router[V]) addEmbeddedParameterNode(current *node[V], part string) *node[V] {
	for {
		if part == "" {
			break
		}

		idx := strings.IndexByte(part, ':')
		if idx == -1 {
			// Remaining part is static
			if current.staticChildren == nil {
				current.staticChildren = make(map[string]*node[V])
			}
			if current.staticChildren[part] == nil {
				current.staticChildren[part] = &node[V]{parent: current}
			}
			current = current.staticChildren[part]
			break
		}

		// Static part before ':'
		if idx > 0 {
			staticPart := part[:idx]
			if current.staticChildren == nil {
				current.staticChildren = make(map[string]*node[V])
			}
			if current.staticChildren[staticPart] == nil {
				current.staticChildren[staticPart] = &node[V]{parent: current}
			}
			current = current.staticChildren[staticPart]
		}

		// Parameter part after ':'
		part = part[idx+1:]
		nextIdx := strings.IndexAny(part, ":*")
		if nextIdx != -1 {
			part = part[nextIdx:]
		} else {
			part = ""
		}
		if current.paramChild == nil {
			current.paramChild = &node[V]{parent: current}
		}
		current = current.paramChild
	}
	return current
}

func (r *Router[V]) buildMiddlewareChain(current *node[V], routeMiddleware []MiddlewareFunc[V]) []MiddlewareFunc[V] {
	var middlewareChain []MiddlewareFunc[V]
	// Collect middleware from nodes
	currentNode := current
	var middlewareStack [][]MiddlewareFunc[V]
	for currentNode != nil {
		if len(currentNode.middleware) > 0 {
			middlewareStack = append(middlewareStack, currentNode.middleware)
		}
		currentNode = currentNode.parent
	}
	// Add router's middleware
	if len(r.middleware) > 0 {
		middlewareStack = append(middlewareStack, r.middleware)
	}
	if len(r.preGroupMiddleware) > 0 {
		middlewareStack = append(middlewareStack, r.preGroupMiddleware)
	}
	// Flatten the middleware stack in the correct order
	for i := len(middlewareStack) - 1; i >= 0; i-- {
		middlewareChain = append(middlewareChain, middlewareStack[i]...)
	}
	// Add route-specific middleware
	if len(routeMiddleware) > 0 {
		middlewareChain = append(middlewareChain, routeMiddleware...)
	}
	return middlewareChain
}

func (r *Router[V]) globalMiddlewareChain() []MiddlewareFunc[V] {
	var middlewareChain []MiddlewareFunc[V]
	if len(r.preGroupMiddleware) > 0 {
		middlewareChain = append(middlewareChain, r.preGroupMiddleware...)
	}
	if len(r.middleware) > 0 {
		middlewareChain = append(middlewareChain, r.middleware...)
	}
	return middlewareChain
}

// splitPath splits the path into segments without allocating unnecessary memory
func splitPath(path string) []string {
	if path == "" || path == "/" {
		return nil
	}
	if path[0] == '/' {
		path = path[1:]
	}
	// Count segments to preallocate slice
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

// search finds the handler and middleware chain for a given request
func (r *Router[V]) search(method, path string) (HandlerFunc[V], []MiddlewareFunc[V], map[string]string, bool) {
	parts := splitPath(path)
	current := r.root

	var paramsValues []string

	for i, part := range parts {
		if part == "" {
			continue
		}

		// First, try to match exact static segments
		if child, ok := current.staticChildren[part]; ok {
			current = child
			continue
		}

		// Next, try to match embedded parameters
		matched := false
		if current.staticChildren != nil {
			for key, child := range current.staticChildren {
				if strings.HasPrefix(part, key) {
					remaining := part[len(key):]
					if remaining != "" {
						current = child
						part = remaining
						for {
							if current.paramChild != nil {
								paramsValues = append(paramsValues, part)
								current = current.paramChild
								matched = true
								break
							}
							if current.staticChildren != nil {
								found := false
								for k, c := range current.staticChildren {
									if strings.HasPrefix(part, k) {
										current = c
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

		// Next, try to match standard parameters
		if current.paramChild != nil {
			paramsValues = append(paramsValues, part)
			current = current.paramChild
			continue
		}

		// Finally, try to match wildcard parameters
		if current.wildcardChild != nil {
			// Remaining parts are matched to wildcard parameter
			remainingParts := strings.Join(parts[i:], "/")
			paramsValues = append(paramsValues, remainingParts)
			current = current.wildcardChild
			break
		}

		// No matching child
		return nil, nil, nil, false
	}

	handlerEntry, ok := current.handlers[method]
	if !ok || !current.isLeaf {
		return nil, nil, nil, false
	}

	// Build the params map
	var params map[string]string
	if len(handlerEntry.paramNames) > 0 {
		params = make(map[string]string, len(handlerEntry.paramNames))
		for i, paramName := range handlerEntry.paramNames {
			if i < len(paramsValues) {
				params[paramName] = paramsValues[i]
			}
		}
	}

	return handlerEntry.handler, handlerEntry.middleware, params, true
}

// wrapMiddleware ensures that middleware checks ctx.done before proceeding
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

// wrapHandler ensures that the handler checks ctx.done before proceeding
func wrapHandler[V any](handler HandlerFunc[V]) HandlerFunc[V] {
	return func(ctx *Ctx[V]) {
		if ctx.done {
			return
		}
		handler(ctx)
	}
}

// applyMiddleware wraps the handler with middleware functions
func applyMiddleware[V any](handler HandlerFunc[V], middleware []MiddlewareFunc[V]) HandlerFunc[V] {
	handler = wrapHandler(handler)
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
		// Build middleware chain for 404 handler
		middlewareChain = r.globalMiddlewareChain()
	}

	ctx := &Ctx[V]{
		ResponseWriter: w,
		Request:        req,
		Params:         params,
		StartTime:      time.Now().UnixNano(),
		UUID:           uuid.NewString(),
		Query:          req.URL.Query(),
	}

	handler = applyMiddleware(handler, middlewareChain)

	handler(ctx)
}

func RecoveryMiddleware[V any]() MiddlewareFunc[V] {
	return func(next HandlerFunc[V]) HandlerFunc[V] {
		return func(ctx *Ctx[V]) {
			defer func() {
				if err := recover(); err != nil {
					// reset response writer
					// Log the error details along with stack trace
					if logger != nil {
						logger.Error().
							Interface("error", err).
							Str("stack", string(debug.Stack())).
							Msgf("[octo-panic] panic recovered: %v", err)
					} else {
						fmt.Printf("[octo-panic] panic recovered: %v\n", err)
					}

					// Return a standard error response
					ctx.SendError("err_internal_error", fmt.Errorf("%v", err))
				}
			}()

			// Proceed to the next handler
			next(ctx)
		}
	}
}
