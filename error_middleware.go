package octo

import (
    "context"
)

// RequestErrorKey is the context key used to attach request information to errors
type RequestErrorKey struct{}

// RequestErrorInfo contains information about the request for error context
type RequestErrorInfo struct {
    Method  string
    Path    string
    IP      string
    Headers map[string]string
}

// ErrorContextMiddleware adds request context to all errors that occur in handlers
func ErrorContextMiddleware[V any]() MiddlewareFunc[V] {
    return func(next HandlerFunc[V]) HandlerFunc[V] {
        return func(ctx *Ctx[V]) {
            // Collect important request information
            info := &RequestErrorInfo{
                Method:  ctx.Request.Method,
                Path:    ctx.Request.URL.Path,
                IP:      ctx.ClientIP(),
                Headers: make(map[string]string),
            }
            
            // Collect important headers but not sensitive ones
            safeHeaders := []string{
                "User-Agent",
                "Accept",
                "Accept-Language",
                "Accept-Encoding",
                "Content-Type",
                "Content-Length",
                "X-Request-ID",
                "X-Correlation-ID",
            }
            
            for _, header := range safeHeaders {
                if value := ctx.GetHeader(header); value != "" {
                    info.Headers[header] = value
                }
            }
            
            // Attach this information to the request context
            ctx.Request = ctx.Request.WithContext(
                context.WithValue(ctx.Request.Context(), RequestErrorKey{}, info),
            )
            
            // Call the next handler
            next(ctx)
        }
    }
}

// EnhanceLogErrorsMiddleware adds additional context from errors
// when they happen to ensure logs contain all valuable information
func EnhanceLogErrorsMiddleware[V any]() MiddlewareFunc[V] {
    return func(next HandlerFunc[V]) HandlerFunc[V] {
        return func(ctx *Ctx[V]) {
            // We can't override the SendError method directly, so we'll add logging here
            // and let the normal SendError method run when called
            
            // Call the next handler
            next(ctx)
            
            // If we're done and there was likely an error (non-200 status),
            // add the enhanced error information
            if ctx.IsDone() && ctx.ResponseWriter.Status >= 400 {
                // Get request info from context if available
                info, ok := ctx.Request.Context().Value(RequestErrorKey{}).(*RequestErrorInfo)
                if ok && logger != nil {
                    // Log with enhanced context
                    if EnableLoggerCheck && logger == nil {
                        // Skip logging if logger is disabled
                    } else {
                        event := logger.Error().
                            Int("status_code", ctx.ResponseWriter.Status).
                            Str("path", info.Path).
                            Str("method", info.Method).
                            Str("ip", info.IP)
                        
                        // Add headers if available
                        for k, v := range info.Headers {
                            event = event.Str("header_"+k, v)
                        }
                        
                        event.Msg("[octo] Request resulted in error")
                    }
                }
            }
        }
    }
}