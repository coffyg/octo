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

// HandlerFunc is the function type for route handlers
type HandlerFunc[V any] func(*Ctx[V])

// MiddlewareFunc is the function type for middleware functions
type MiddlewareFunc[V any] func(HandlerFunc[V]) HandlerFunc[V]

// routeEntry represents a route definition with handler and its middleware
type routeEntry[V any] struct {
    handler    HandlerFunc[V]
    paramNames []string
    middleware []MiddlewareFunc[V]
}

// node represents a node in the router's trie structure
type node[V any] struct {
    staticChildren map[string]*node[V]
    paramChild     *node[V]
    wildcardChild  *node[V]
    isLeaf         bool
    handlers       map[string]*routeEntry[V]
    middleware     []MiddlewareFunc[V]
    parent         *node[V]
}

// Router is the main HTTP router implementation
type Router[V any] struct {
    root               *node[V]
    middleware         []MiddlewareFunc[V]
    preGroupMiddleware []MiddlewareFunc[V]
}

// NewRouter creates a new HTTP router instance
func NewRouter[V any]() *Router[V] {
    return &Router[V]{
        root: &node[V]{},
    }
}

// UseGlobal adds middleware that applies to all routes before group middleware
func (r *Router[V]) UseGlobal(mw MiddlewareFunc[V]) {
    r.preGroupMiddleware = append(r.preGroupMiddleware, mw)
}

// Use adds global middleware to the router that applies after group middleware
func (r *Router[V]) Use(mw MiddlewareFunc[V]) {
    r.middleware = append(r.middleware, mw)
}

// GET registers a route that matches GET HTTP method
func (r *Router[V]) GET(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    r.addRoute("GET", path, handler, middleware...)
}

// POST registers a route that matches POST HTTP method
func (r *Router[V]) POST(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    r.addRoute("POST", path, handler, middleware...)
}

// PUT registers a route that matches PUT HTTP method
func (r *Router[V]) PUT(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    r.addRoute("PUT", path, handler, middleware...)
}

// DELETE registers a route that matches DELETE HTTP method
func (r *Router[V]) DELETE(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    r.addRoute("DELETE", path, handler, middleware...)
}

// PATCH registers a route that matches PATCH HTTP method
func (r *Router[V]) PATCH(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    r.addRoute("PATCH", path, handler, middleware...)
}

// OPTIONS registers a route that matches OPTIONS HTTP method
func (r *Router[V]) OPTIONS(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    r.addRoute("OPTIONS", path, handler, middleware...)
}

// HEAD registers a route that matches HEAD HTTP method
func (r *Router[V]) HEAD(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    r.addRoute("HEAD", path, handler, middleware...)
}

// ANY registers a route that matches all HTTP methods
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

// Use adds middleware to the route group
func (g *Group[V]) Use(mw MiddlewareFunc[V]) {
    g.middleware = append(g.middleware, mw)
}

// Group creates a new route group with the given prefix and middleware
func (r *Router[V]) Group(prefix string, middleware ...MiddlewareFunc[V]) *Group[V] {
    // Add nodes for the group prefix path
    current := r.root
    parts := splitPath(prefix)
    
    for _, part := range parts {
        if part == "" {
            continue
        }
        
        if part[0] == ':' || strings.Contains(part, ":") {
            // Parameter node
            current = r.addEmbeddedParameterNode(current, part)
        } else {
            // Static node
            if current.staticChildren == nil {
                current.staticChildren = make(map[string]*node[V])
            }
            
            if current.staticChildren[part] == nil {
                current.staticChildren[part] = &node[V]{parent: current}
            }
            
            current = current.staticChildren[part]
        }
    }
    
    // Create and return the group
    return &Group[V]{
        prefix:     prefix,
        router:     r,
        middleware: middleware,
    }
}

// GET registers a route in the group that matches GET HTTP method
func (g *Group[V]) GET(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    fullPath := g.prefix + path
    allMiddleware := append(g.middleware, middleware...)
    g.router.GET(fullPath, handler, allMiddleware...)
}

// POST registers a route in the group that matches POST HTTP method
func (g *Group[V]) POST(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    fullPath := g.prefix + path
    allMiddleware := append(g.middleware, middleware...)
    g.router.POST(fullPath, handler, allMiddleware...)
}

// PUT registers a route in the group that matches PUT HTTP method
func (g *Group[V]) PUT(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    fullPath := g.prefix + path
    allMiddleware := append(g.middleware, middleware...)
    g.router.PUT(fullPath, handler, allMiddleware...)
}

// DELETE registers a route in the group that matches DELETE HTTP method
func (g *Group[V]) DELETE(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    fullPath := g.prefix + path
    allMiddleware := append(g.middleware, middleware...)
    g.router.DELETE(fullPath, handler, allMiddleware...)
}

// PATCH registers a route in the group that matches PATCH HTTP method
func (g *Group[V]) PATCH(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    fullPath := g.prefix + path
    allMiddleware := append(g.middleware, middleware...)
    g.router.PATCH(fullPath, handler, allMiddleware...)
}

// OPTIONS registers a route in the group that matches OPTIONS HTTP method
func (g *Group[V]) OPTIONS(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    fullPath := g.prefix + path
    allMiddleware := append(g.middleware, middleware...)
    g.router.OPTIONS(fullPath, handler, allMiddleware...)
}

// HEAD registers a route in the group that matches HEAD HTTP method
func (g *Group[V]) HEAD(path string, handler HandlerFunc[V], middleware ...MiddlewareFunc[V]) {
    fullPath := g.prefix + path
    allMiddleware := append(g.middleware, middleware...)
    g.router.HEAD(fullPath, handler, allMiddleware...)
}

// ANY registers a route in the group that matches all HTTP methods
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

    // Process each path segment
    for i, part := range parts {
        if part == "" {
            continue
        }
        
        if strings.Contains(part, ":") {
            // Parameter segment with embedded parameters
            current, paramNames = r.addEmbeddedParameterNodeWithNames(current, part, paramNames)
        } else if part[0] == '*' {
            // Wildcard segment
            current, paramNames = r.addWildcardNode(current, part, paramNames, i, parts)
        } else {
            // Static segment
            current = r.addStaticNode(current, part)
        }
    }

    // Create route entry for the path
    r.createRouteEntry(current, method, path, handler, paramNames, routeMW)
}

// addWildcardNode adds a wildcard node to the router trie
func (r *Router[V]) addWildcardNode(
    current *node[V], 
    part string, 
    paramNames []string, 
    index int, 
    parts []string,
) (*node[V], []string) {
    // Extract parameter name from wildcard
    paramName := part[1:]
    paramNames = append(paramNames, paramName)
    
    // Create wildcard child if it doesn't exist
    if current.wildcardChild == nil {
        current.wildcardChild = &node[V]{parent: current}
    }
    
    // Ensure wildcard is the last segment in the path
    if index != len(parts)-1 {
        panic("Wildcard route parameter must be at the end of the path")
    }
    
    return current.wildcardChild, paramNames
}

// addStaticNode adds a static node to the router trie
func (r *Router[V]) addStaticNode(current *node[V], part string) *node[V] {
    if current.staticChildren == nil {
        current.staticChildren = make(map[string]*node[V])
    }
    
    if current.staticChildren[part] == nil {
        current.staticChildren[part] = &node[V]{parent: current}
    }
    
    return current.staticChildren[part]
}

// createRouteEntry creates a route entry in the given node
func (r *Router[V]) createRouteEntry(
    current *node[V], 
    method string, 
    path string, 
    handler HandlerFunc[V], 
    paramNames []string, 
    routeMW []MiddlewareFunc[V],
) {
    // Initialize handlers map if needed
    if current.handlers == nil {
        current.handlers = make(map[string]*routeEntry[V])
    }

    // Check for duplicate route
    if _, exists := current.handlers[method]; exists {
        panic(fmt.Sprintf("route already defined: %s %s", method, path))
    }

    // Mark as leaf node
    current.isLeaf = true

    // Build middleware chain
    middlewareChain := r.buildMiddlewareChain(current, routeMW)
    
    // Create route entry
    current.handlers[method] = &routeEntry[V]{
        handler:    handler,
        paramNames: paramNames,
        middleware: middlewareChain,
    }
}

// addEmbeddedParameterNodeWithNames processes path segments with embedded parameters
// and extracts parameter names
func (r *Router[V]) addEmbeddedParameterNodeWithNames(
    current *node[V], 
    part string, 
    paramNames []string,
) (*node[V], []string) {
    for {
        // Exit when done processing the part
        if part == "" {
            break
        }
        
        // Find the start of a parameter
        idx := strings.IndexByte(part, ':')
        
        if idx == -1 {
            // No more parameters, remaining part is static
            current = r.addTerminalStaticPart(current, part)
            break
        }
        
        if idx > 0 {
            // Handle static part before parameter
            staticPart := part[:idx]
            current = r.addStaticPartBeforeParam(current, staticPart)
        }
        
        // Process parameter part
        part = part[idx+1:]
        paramName, remainingPart := r.extractParamName(part)
        paramNames = append(paramNames, paramName)
        part = remainingPart
        
        // Add parameter child node
        if current.paramChild == nil {
            current.paramChild = &node[V]{parent: current}
        }
        current = current.paramChild
    }
    
    return current, paramNames
}

// addTerminalStaticPart adds a static part at the end of a path segment
func (r *Router[V]) addTerminalStaticPart(current *node[V], part string) *node[V] {
    if current.staticChildren == nil {
        current.staticChildren = make(map[string]*node[V])
    }
    
    if current.staticChildren[part] == nil {
        current.staticChildren[part] = &node[V]{parent: current}
    }
    
    return current.staticChildren[part]
}

// addStaticPartBeforeParam adds a static part before a parameter
func (r *Router[V]) addStaticPartBeforeParam(current *node[V], staticPart string) *node[V] {
    if current.staticChildren == nil {
        current.staticChildren = make(map[string]*node[V])
    }
    
    if current.staticChildren[staticPart] == nil {
        current.staticChildren[staticPart] = &node[V]{parent: current}
    }
    
    return current.staticChildren[staticPart]
}

// extractParamName extracts a parameter name from a path segment
func (r *Router[V]) extractParamName(part string) (string, string) {
    nextIdx := strings.IndexAny(part, ":*")
    
    if nextIdx != -1 {
        return part[:nextIdx], part[nextIdx:]
    }
    
    return part, ""
}

// addEmbeddedParameterNode processes path segments with embedded parameters
// but does not extract parameter names
func (r *Router[V]) addEmbeddedParameterNode(current *node[V], part string) *node[V] {
    for {
        // Exit when done processing the part
        if part == "" {
            break
        }
        
        // Find the start of a parameter
        idx := strings.IndexByte(part, ':')
        
        if idx == -1 {
            // No more parameters, remaining part is static
            current = r.addTerminalStaticPart(current, part)
            break
        }
        
        if idx > 0 {
            // Handle static part before parameter
            staticPart := part[:idx]
            current = r.addStaticPartBeforeParam(current, staticPart)
        }
        
        // Process parameter part
        part = part[idx+1:]
        _, remainingPart := r.extractParamName(part)
        part = remainingPart
        
        // Add parameter child node
        if current.paramChild == nil {
            current.paramChild = &node[V]{parent: current}
        }
        current = current.paramChild
    }
    
    return current
}

// buildMiddlewareChain constructs the middleware chain for a route
func (r *Router[V]) buildMiddlewareChain(current *node[V], routeMW []MiddlewareFunc[V]) []MiddlewareFunc[V] {
    // Start with global middleware
    var chain []MiddlewareFunc[V]
    chain = append(chain, r.preGroupMiddleware...)
    chain = append(chain, r.middleware...)

    // Collect and add middleware from parent nodes
    nodeMW := r.collectNodeMiddleware(current)
    chain = append(chain, nodeMW...)
    
    // Add route-specific middleware
    chain = append(chain, routeMW...)
    
    return chain
}

// collectNodeMiddleware collects middleware from the node hierarchy
func (r *Router[V]) collectNodeMiddleware(current *node[V]) []MiddlewareFunc[V] {
    // Collect middleware from parent nodes
    var nodeMW []MiddlewareFunc[V]
    temp := current
    
    // Traverse up the tree
    for temp != nil {
        if len(temp.middleware) > 0 {
            nodeMW = append(nodeMW, temp.middleware...)
        }
        temp = temp.parent
    }
    
    // If no middleware, return nil
    if len(nodeMW) == 0 {
        return nil
    }
    
    // Reverse middleware order so parent-most is first
    var reversedMW []MiddlewareFunc[V]
    for i := len(nodeMW) - 1; i >= 0; i-- {
        reversedMW = append(reversedMW, nodeMW[i])
    }
    
    return reversedMW
}

// globalMiddlewareChain returns the global middleware chain
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

// splitPath splits a URL path into segments
func splitPath(path string) []string {
    // Handle empty paths
    if path == "" || path == "/" {
        return nil
    }
    
    // Remove leading slash
    if path[0] == '/' {
        path = path[1:]
    }
    
    // Pre-calculate segment count for capacity
    segmentCount := 1
    for i := 0; i < len(path); i++ {
        if path[i] == '/' {
            segmentCount++
        }
    }
    
    // Split the path
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

    // Apply security headers if enabled
    r.addSecurityHeaders(w)

    // Find matching route or use not found handler
    handler, middlewareChain, params, found := r.search(method, path)
    
    if !found {
        handler = r.createNotFoundHandler(req, w)
        middlewareChain = r.globalMiddlewareChain()
    }

    // Create response writer wrapper
    responseWriter := NewResponseWriterWrapper(w)

    // Create context
    ctx := &Ctx[V]{
        ResponseWriter: responseWriter,
        Request:        req,
        Params:         params,
        StartTime:      time.Now().UnixNano(),
        UUID:           uuid.NewString(),
        Query:          req.URL.Query(),
    }

    // Apply middleware and execute handler
    handler = applyMiddleware(handler, middlewareChain)
    handler(ctx)
}

// addSecurityHeaders adds security headers to the response if enabled
func (r *Router[V]) addSecurityHeaders(w http.ResponseWriter) {
    if EnableSecurityHeaders {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
    }
}

// createNotFoundHandler creates a handler for non-matching routes
func (r *Router[V]) createNotFoundHandler(req *http.Request, w http.ResponseWriter) HandlerFunc[V] {
    return func(ctx *Ctx[V]) {
        // Handle OPTIONS requests with allowed methods
        if req.Method == "OPTIONS" {
            w.Header().Set("Allow", "GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD")
            w.WriteHeader(http.StatusOK)
            return
        }
        
        // Return 404 for all other methods
        http.NotFound(ctx.ResponseWriter, ctx.Request)
    }
}

// search finds a matching route for the given method and path
func (r *Router[V]) search(method, path string) (HandlerFunc[V], []MiddlewareFunc[V], map[string]string, bool) {
    parts := splitPath(path)
    current := r.root
    var paramValues []string

    // Process path segments
    for i, part := range parts {
        if part == "" {
            continue
        }
        
        // Try static child match
        if child, ok := current.staticChildren[part]; ok {
            current = child
            continue
        }

        // Try embedded parameter match
        matched := r.tryMatchEmbeddedParam(current, part, &paramValues)
        if matched != nil {
            current = matched
            continue
        }

        // Try parameter match
        if current.paramChild != nil {
            paramValues = append(paramValues, part)
            current = current.paramChild
            continue
        }
        
        // Try wildcard match
        if current.wildcardChild != nil {
            remainingParts := strings.Join(parts[i:], "/")
            paramValues = append(paramValues, remainingParts)
            current = current.wildcardChild
            break
        }
        
        // No match found
        return nil, nil, nil, false
    }

    // Check if the current node has a handler for the requested method
    return r.createHandlerFromMatch(current, method, paramValues)
}

// tryMatchEmbeddedParam attempts to match a path segment with embedded parameters
func (r *Router[V]) tryMatchEmbeddedParam(
    current *node[V], 
    part string, 
    paramValues *[]string,
) *node[V] {
    if current.staticChildren == nil {
        return nil
    }
    
    // Check all static children for partial matches
    for key, child := range current.staticChildren {
        if strings.HasPrefix(part, key) {
            remaining := part[len(key):]
            if remaining == "" {
                return child
            }
            
            // Try to match the remaining part
            matchedNode := r.matchRemainingPart(child, remaining, paramValues)
            if matchedNode != nil {
                return matchedNode
            }
        }
    }
    
    return nil
}

// matchRemainingPart tries to match the remaining part of a path segment
func (r *Router[V]) matchRemainingPart(
    node *node[V], 
    part string, 
    paramValues *[]string,
) *node[V] {
    current := node
    remaining := part
    
    for {
        // Try parameter match
        if current.paramChild != nil {
            *paramValues = append(*paramValues, remaining)
            return current.paramChild
        }
        
        // Try static match
        if current.staticChildren != nil {
            matched := false
            for k, child := range current.staticChildren {
                if strings.HasPrefix(remaining, k) {
                    current = child
                    remaining = remaining[len(k):]
                    matched = true
                    break
                }
            }
            
            if !matched {
                break
            }
        } else {
            break
        }
    }
    
    return nil
}

// createHandlerFromMatch creates a handler from a matched route
func (r *Router[V]) createHandlerFromMatch(
    node *node[V], 
    method string, 
    paramValues []string,
) (HandlerFunc[V], []MiddlewareFunc[V], map[string]string, bool) {
    // Check if this is a valid leaf node with a handler for the requested method
    handlerEntry, ok := node.handlers[method]
    if !ok || !node.isLeaf {
        return nil, nil, nil, false
    }
    
    // Map parameter values to parameter names
    params := r.createParamsMap(handlerEntry.paramNames, paramValues)
    
    return handlerEntry.handler, handlerEntry.middleware, params, true
}

// createParamsMap creates a map of parameter names to values
func (r *Router[V]) createParamsMap(
    paramNames []string, 
    paramValues []string,
) map[string]string {
    if len(paramNames) == 0 {
        return nil
    }
    
    params := make(map[string]string, len(paramNames))
    for i, paramName := range paramNames {
        if i < len(paramValues) {
            params[paramName] = paramValues[i]
        }
    }
    
    return params
}

// wrapMiddleware ensures middleware doesn't execute if the context is marked as done
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

// wrapHandler ensures handlers don't execute if the context is marked as done
func wrapHandler[V any](handler HandlerFunc[V]) HandlerFunc[V] {
    return func(ctx *Ctx[V]) {
        if ctx.done {
            return
        }
        handler(ctx)
    }
}

// applyMiddleware chains middleware functions around a handler
func applyMiddleware[V any](handler HandlerFunc[V], middleware []MiddlewareFunc[V]) HandlerFunc[V] {
    // Wrap the handler to check for done context
    handler = wrapHandler(handler)
    
    // Apply middleware in reverse order (last middleware runs first)
    for i := len(middleware) - 1; i >= 0; i-- {
        mw := wrapMiddleware(middleware[i])
        handler = mw(handler)
    }
    
    return handler
}

// RecoveryMiddleware returns middleware that recovers from panics
func RecoveryMiddleware[V any]() MiddlewareFunc[V] {
    return func(next HandlerFunc[V]) HandlerFunc[V] {
        return func(ctx *Ctx[V]) {
            defer func() {
                if recovered := recover(); recovered != nil {
                    // Capture stack trace
                    stack := captureStackTrace()
                    stackBytes := []byte(strings.Join(stack, "\n"))
                    
                    // Create a proper OctoError
                    var err error
                    switch v := recovered.(type) {
                    case error:
                        err = v
                    case string:
                        err = errors.New(v)
                    default:
                        err = errors.Errorf("%v", recovered)
                    }
                    
                    // Wrap as OctoError
                    octoErr := Wrap(err, ErrInternal, "Panic recovered")
                    
                    // Handle client aborts differently
                    if errors.Is(err, http.ErrAbortHandler) {
                        logClientAbort(ctx)
                        return
                    }
                    
                    // Log the panic
                    LogPanic(logger, recovered, stackBytes)
                    
                    // Send error response
                    if !ctx.ResponseWriter.Written() {
                        ctx.SendError(string(ErrInternal), octoErr)
                    }
                }
            }()
            next(ctx)
        }
    }
}

// captureStackTrace captures the stack trace when a panic occurs
func captureStackTrace() []string {
    var pcs [32]uintptr
    n := runtime.Callers(3, pcs[:])
    frames := runtime.CallersFrames(pcs[:n])

    var stackLines []string
    for {
        frame, more := frames.Next()
        stackLines = append(stackLines, fmt.Sprintf("%s\n\t%s:%d", 
            frame.Function, frame.File, frame.Line))
        if !more {
            break
        }
    }
    
    return stackLines
}

// createZerologStackArray creates a zerolog array from stack frames
func createZerologStackArray(stackLines []string) *zerolog.Array {
    zStack := zerolog.Arr()
    for _, line := range stackLines {
        zStack.Str(line)
    }
    return zStack
}

// logClientAbort logs when a client aborts a request
func logClientAbort[V any](ctx *Ctx[V]) {
    if EnableLoggerCheck && logger == nil {
        return
    }
    
    logger.Warn().
        Str("path", ctx.Request.URL.Path).
        Str("method", ctx.Request.Method).
        Msg("[octo-panic] Client aborted request (panic recovered)")
}
